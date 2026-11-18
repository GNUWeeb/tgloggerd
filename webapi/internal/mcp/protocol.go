// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

// Package mcp is the Model Context Protocol endpoint's engine: JSON-RPC
// 2.0 framing, the MCP methods a tools-only server answers, and the
// curated Telegram tools. It is the port of the Drogon web's gwmcp
// library (src/gwmcp) and of web/src/mcp, with the same methods, tools,
// arguments, answers and error messages; web/docs/mcp.md describes
// them. The HTTP transport and the bearer tokens are in
// internal/server/mcp.go and token.go.
//
// Nothing here reads a group that is not on the curated exposure list,
// telegram_public_groups: every message query goes through msgGate and
// every group query carries the list as a condition (tools.go).
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
)

// The protocol versions: the latest is answered when the client asks
// for one this server does not know, the previous is accepted.
const (
	ProtocolLatest   = "2025-06-18"
	ProtocolPrevious = "2025-03-26"
)

// JSON-RPC 2.0 error codes.
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// RPCError is a malformed call: unknown method, unknown tool, bad
// params. It becomes a JSON-RPC error.
type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string { return e.Message }

// ToolError is a call that was fine but whose work failed: no such
// user, a filter the caller can fix, a query that timed out. It becomes
// a result with isError true, which a model can read and act on.
type ToolError struct{ Message string }

func (e *ToolError) Error() string { return e.Message }

func toolErr(msg string) error { return &ToolError{Message: msg} }

// Tool is one callable tool.
type Tool struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]any
	// Handler gets the arguments object (empty when none were sent)
	// and returns the result payload, which is placed in
	// structuredContent and, serialised, in content[0].
	Handler func(ctx context.Context, args map[string]any) (any, error)
}

// ServerInfo is what initialize reports about the server.
type ServerInfo struct {
	Name, Title, Version, Instructions string
}

// Server answers MCP messages. It is immutable once built and safe for
// concurrent use.
type Server struct {
	info  ServerInfo
	tools map[string]*Tool
	names []string // sorted, the order of tools/list
}

// NewServer builds a server with the given tools; a tool without a
// name or handler, or with a name already taken, is left out, as
// gwmcp's registry leaves it out.
func NewServer(info ServerInfo, tools []Tool) *Server {
	s := &Server{info: info, tools: map[string]*Tool{}}
	for i := range tools {
		t := &tools[i]
		if t.Name == "" || t.Handler == nil || s.tools[t.Name] != nil {
			continue
		}
		s.tools[t.Name] = t
		s.names = append(s.names, t.Name)
	}
	sort.Strings(s.names)
	return s
}

// HandleRaw answers one HTTP body. ok is false when no reply is due (a
// notification, or a response sent by the client), which the transport
// answers with 202 and no body.
func (s *Server) HandleRaw(ctx context.Context, body []byte) (reply []byte, ok bool) {
	msg, err := decode(body)
	if err != nil {
		// A parse failure has no id to answer to.
		return encode(rpcErrorReply(nil, ParseError, "Parse error: "+err.Error())), true
	}
	// Batches were removed from the protocol in 2025-06-18.
	if _, batch := msg.([]any); batch {
		return encode(rpcErrorReply(nil, InvalidRequest, "Batch requests are not supported")), true
	}
	r, ok := s.handle(ctx, msg)
	if !ok {
		return nil, false
	}
	return encode(r), true
}

// RPCErrorBody is the JSON of a JSON-RPC error without an id, for the
// transport's own refusals (401, 403, 400).
func RPCErrorBody(code int, msg string) []byte {
	return encode(rpcErrorReply(nil, code, msg))
}

func rpcResult(id, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func rpcErrorReply(id any, code int, msg string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	}
}

// messageID is the request's id when it is one JSON-RPC allows (a
// string or a number), else null.
func messageID(m any) any {
	o, ok := m.(map[string]any)
	if !ok {
		return nil
	}
	switch id := o["id"].(type) {
	case string:
		return id
	case json.Number:
		return id
	default:
		return nil
	}
}

// isNotification: an object without an id, or with a null one, which
// some clients send and which nothing can be correlated to.
func isNotification(m any) bool {
	o, ok := m.(map[string]any)
	if !ok {
		return false
	}
	return o["id"] == nil
}

func validateEnvelope(m any) string {
	o, ok := m.(map[string]any)
	if !ok {
		return "message must be a JSON object"
	}
	if v, ok := o["jsonrpc"].(string); !ok || v != "2.0" {
		return `missing or invalid "jsonrpc": expected "2.0"`
	}
	if _, ok := o["method"].(string); !ok {
		return `missing or invalid "method"`
	}
	if p, ok := o["params"]; ok {
		switch p.(type) {
		case map[string]any, []any, nil:
		default:
			return `"params" must be an object or an array`
		}
	}
	return ""
}

func (s *Server) handle(ctx context.Context, m any) (any, bool) {
	id := messageID(m)
	notification := isNotification(m)
	if err := validateEnvelope(m); err != "" {
		if notification {
			return nil, false
		}
		return rpcErrorReply(id, InvalidRequest, err), true
	}
	o := m.(map[string]any)
	method := o["method"].(string)
	params, _ := o["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}

	// Notifications get no reply, not even an error, so an unknown one
	// is quietly accepted, as the spec expects.
	if notification {
		return nil, false
	}
	var (
		result any
		err    error
	)
	switch method {
	case "initialize":
		result = s.initializeResult(params)
	case "ping":
		result = map[string]any{}
	case "tools/list":
		// No pagination: the tool set is small and fixed.
		result = map[string]any{"tools": s.listTools()}
	case "tools/call":
		result, err = s.callTool(ctx, params)
	default:
		return rpcErrorReply(id, MethodNotFound, "Unknown method: "+method), true
	}
	if err != nil {
		var re *RPCError
		if errors.As(err, &re) {
			return rpcErrorReply(id, re.Code, re.Message), true
		}
		return rpcErrorReply(id, InternalError, err.Error()), true
	}
	return rpcResult(id, result), true
}

func (s *Server) initializeResult(params map[string]any) map[string]any {
	// Negotiation: echo the client's version when known, else answer
	// with the latest and let the client decide. Not an error.
	agreed := ProtocolLatest
	if v, ok := params["protocolVersion"].(string); ok &&
		(v == ProtocolLatest || v == ProtocolPrevious) {
		agreed = v
	}
	si := map[string]any{"name": s.info.Name, "version": s.info.Version}
	if s.info.Title != "" {
		si["title"] = s.info.Title
	}
	r := map[string]any{
		"protocolVersion": agreed,
		// listChanged false: the tool set never changes while running.
		"capabilities": map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":   si,
	}
	if s.info.Instructions != "" {
		r["instructions"] = s.info.Instructions
	}
	return r
}

func (s *Server) listTools() []any {
	out := make([]any, 0, len(s.names))
	for _, n := range s.names {
		t := s.tools[n]
		j := map[string]any{"name": t.Name}
		if t.Title != "" {
			j["title"] = t.Title
		}
		if t.Description != "" {
			j["description"] = t.Description
		}
		if t.InputSchema != nil {
			j["inputSchema"] = t.InputSchema
		} else {
			j["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		j["annotations"] = map[string]any{"readOnlyHint": true}
		out = append(out, j)
	}
	return out
}

func toolFailure(text string) map[string]any {
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
		"isError": true,
	}
}

func (s *Server) callTool(ctx context.Context, params map[string]any) (any, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, &RPCError{InvalidParams, `tools/call requires a string "name"`}
	}
	t := s.tools[name]
	if t == nil {
		return nil, &RPCError{InvalidParams, "Unknown tool: " + name}
	}
	args := map[string]any{}
	if a, present := params["arguments"]; present {
		switch v := a.(type) {
		case map[string]any:
			args = v
		case nil:
		default:
			return nil, &RPCError{InvalidParams, `"arguments" must be an object`}
		}
	}

	payload, err := t.Handler(ctx, args)
	if err != nil {
		var re *RPCError
		if errors.As(err, &re) {
			return nil, err
		}
		var te *ToolError
		if errors.As(err, &te) {
			return toolFailure(te.Message), nil
		}
		// Any other failure is still the tool's, never the transport's.
		return toolFailure("Tool failed: " + err.Error()), nil
	}
	// The payload twice: structuredContent is the answer, content[0]
	// the same serialised, for clients older than structured content.
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(encode(payload))}},
		"structuredContent": payload,
		"isError":           false,
	}, nil
}
