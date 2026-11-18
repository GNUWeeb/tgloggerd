// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// Ownership (web migration 000008, docs/design/web-accounts.md section
// 6): web_resource_relations records which account holds which
// relation to a resource; "owner" is the only relation so far. Most
// resources live in the logger's schema, so nothing ties a row to its
// resource but these helpers: the operation that creates a resource
// sets its owner, the one that deletes it drops its relations. A
// resource with no owner row counts as owned by the superadmins, which
// is what every resource was before ownership existed.
//
// Every change writes a web_audit row in the same transaction:
// owner_set (a first owner, at creation or by the assign-owner
// command), owner_transfer, and resource_drop.

// ResourceType names the table a resource id points into. The values
// are stored; never rename one.
type ResourceType string

const (
	// ResMCPGroup is a group the MCP endpoint exposes,
	// telegram_public_groups.group_id (a negative chat id).
	ResMCPGroup ResourceType = "mcp_group"
	// ResMCPToken is an MCP bearer token, web_mcp_tokens.id.
	ResMCPToken ResourceType = "mcp_token"
	// ResTelegramBot is a bot Discord to Telegram routes send through,
	// telegram_bots.id.
	ResTelegramBot ResourceType = "telegram_bot"
	// ResDiscordBot is a Discord bot; its table comes with
	// discord-bots-per-user, the name is reserved here.
	ResDiscordBot ResourceType = "discord_bot"
	// ResT2DWebhook is a Telegram to Discord webhook,
	// telegram_discord_webhooks.id.
	ResT2DWebhook ResourceType = "t2d_webhook"
	// ResD2TRoute is a Discord to Telegram route,
	// discord_telegram_routes.id.
	ResD2TRoute ResourceType = "d2t_route"
)

// ResourceTypes are the known types, in the order reports list them.
var ResourceTypes = []ResourceType{ResMCPGroup, ResMCPToken, ResTelegramBot, ResDiscordBot,
	ResT2DWebhook, ResD2TRoute}

func (t ResourceType) valid() bool {
	for _, k := range ResourceTypes {
		if t == k {
			return true
		}
	}
	return false
}

// Relation is what an account is to a resource.
type Relation string

// Owner is the account a resource belongs to: one per resource.
const Owner Relation = "owner"

// Resource is one resource: its type and its id in that type's table.
type Resource struct {
	Type ResourceType
	ID   int64
}

func (r Resource) String() string {
	return fmt.Sprintf("%s %d", r.Type, r.ID)
}

// Actor is who makes an ownership change, for created_by and the audit
// row: an account and the address it came from, or, with a nil ID, the
// assign-owner command run at deployment.
type Actor struct {
	ID *uint64
	IP string
}

// By is the actor for account uid at address ip.
func By(uid uint64, ip string) Actor {
	return Actor{ID: &uid, IP: ip}
}

// Deployment is the actor of the assign-owner command.
var Deployment = Actor{}

func (a Actor) id() any {
	if a.ID == nil {
		return nil
	}
	return *a.ID
}

var (
	// ErrUnknownType: the resource type is not one of ResourceTypes.
	ErrUnknownType = errors.New("unknown resource type")
	// ErrOwned: the resource has an owner already; use TransferOwner.
	ErrOwned = errors.New("the resource has an owner")
	// ErrNotOwned: the resource has no owner to transfer from.
	ErrNotOwned = errors.New("the resource has no owner")
)

// OwnerInfo is a resource's owner with what the pages show of it.
type OwnerInfo struct {
	ID          uint64
	Username    string
	DisplayName string
	Role        string
	Active      bool
}

// Name is how the pages name the owner: its display name, or its
// username when it has none.
func (o OwnerInfo) Name() string {
	if o.DisplayName != "" {
		return o.DisplayName
	}
	return o.Username
}

// Owners is the owner of each of ids of type t that has one; an id
// without an owner is not in the map. One query, for a whole list.
func Owners(ctx context.Context, db *sql.DB, t ResourceType, ids []int64) (map[int64]OwnerInfo, error) {
	if !t.valid() {
		return nil, ErrUnknownType
	}
	out := map[int64]OwnerInfo{}
	if len(ids) == 0 {
		return out, nil
	}
	args := []any{string(t), string(Owner)}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.QueryContext(ctx, "SELECT r.resource_id, u.id, u.username, u.display_name, "+
		"u.role, u.is_active FROM web_resource_relations r JOIN web_users u ON u.id = r.user_id "+
		"WHERE r.resource_type = ? AND r.relation = ? AND r.resource_id IN (?"+
		strings.Repeat(", ?", len(ids)-1)+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id int64
			o  OwnerInfo
		)
		if err := rows.Scan(&id, &o.ID, &o.Username, &o.DisplayName, &o.Role, &o.Active); err != nil {
			return nil, err
		}
		out[id] = o
	}
	return out, rows.Err()
}

// OwnerOf is a resource's owner; false when it has none.
func OwnerOf(ctx context.Context, db *sql.DB, r Resource) (OwnerInfo, bool, error) {
	m, err := Owners(ctx, db, r.Type, []int64{r.ID})
	if err != nil {
		return OwnerInfo{}, false, err
	}
	o, ok := m[r.ID]
	return o, ok, nil
}

// OwnedBy is the ids of type t that account uid owns, in ascending
// order: what a per-owner list shows a member.
func OwnedBy(ctx context.Context, db *sql.DB, t ResourceType, uid uint64) ([]int64, error) {
	if !t.valid() {
		return nil, ErrUnknownType
	}
	rows, err := db.QueryContext(ctx, "SELECT resource_id FROM web_resource_relations "+
		"WHERE user_id = ? AND relation = ? AND resource_type = ? ORDER BY resource_id",
		uid, string(Owner), string(t))
	if err != nil {
		return nil, err
	}
	return scanIDs(rows)
}

func scanIDs(rows *sql.Rows) ([]int64, error) {
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// auditOwner writes the audit row of an ownership change: actor, the
// owner it concerns (target) and "<type> <id>" with more detail.
func auditOwner(ctx context.Context, tx *sql.Tx, by Actor, target *uint64, action string,
	r Resource, detail string) error {
	d := r.String()
	if detail != "" {
		d += " " + detail
	}
	return audit(ctx, tx, by.ID, target, action, by.IP, d)
}

// SetOwner gives a resource that has no owner to account owner, as the
// operation creating it does. ErrOwned when it has one already.
func SetOwner(ctx context.Context, db *sql.DB, r Resource, owner uint64, by Actor) error {
	if !r.Type.valid() {
		return ErrUnknownType
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := setOwner(ctx, tx, r, owner, by, ""); err != nil {
		return err
	}
	return tx.Commit()
}

func setOwner(ctx context.Context, tx *sql.Tx, r Resource, owner uint64, by Actor, detail string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO web_resource_relations (resource_type, "+
		"resource_id, relation, user_id, created_by) VALUES (?, ?, ?, ?, ?)",
		string(r.Type), r.ID, string(Owner), owner, by.id())
	if isDuplicate(err) {
		return ErrOwned
	}
	if err != nil {
		return err
	}
	return auditOwner(ctx, tx, by, &owner, "owner_set", r, detail)
}

// TransferOwner gives an owned resource to account to and returns the
// owner it had. Transferring to the owner it has changes nothing and is
// not audited. ErrNotOwned when it has no owner: a resource that counts
// as the superadmins' gets its first owner through SetOwner. Who may
// transfer to whom is the caller's to check (web-accounts.md section
// 6), as is whether the account is active.
func TransferOwner(ctx context.Context, db *sql.DB, r Resource, to uint64, by Actor) (uint64, error) {
	if !r.Type.valid() {
		return 0, ErrUnknownType
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var from uint64
	err = tx.QueryRowContext(ctx, "SELECT user_id FROM web_resource_relations "+
		"WHERE resource_type = ? AND resource_id = ? AND relation = ? FOR UPDATE",
		string(r.Type), r.ID, string(Owner)).Scan(&from)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotOwned
	}
	if err != nil {
		return 0, err
	}
	if from == to {
		return from, nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE web_resource_relations SET user_id = ?, "+
		"created_at = NOW(), created_by = ? WHERE resource_type = ? AND resource_id = ? "+
		"AND relation = ?", to, by.id(), string(r.Type), r.ID, string(Owner)); err != nil {
		return 0, err
	}
	if err := auditOwner(ctx, tx, by, &to, "owner_transfer", r,
		fmt.Sprintf("from %d to %d", from, to)); err != nil {
		return 0, err
	}
	return from, tx.Commit()
}

// DropResource removes every relation of a resource that is being
// deleted, so its id, if the table ever hands it out again, starts
// with no owner. A resource with no relations changes nothing and is
// not audited.
func DropResource(ctx context.Context, db *sql.DB, r Resource, by Actor) error {
	if !r.Type.valid() {
		return ErrUnknownType
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "DELETE FROM web_resource_relations "+
		"WHERE resource_type = ? AND resource_id = ?", string(r.Type), r.ID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return err
	}
	if err := auditOwner(ctx, tx, by, nil, "resource_drop", r, ""); err != nil {
		return err
	}
	return tx.Commit()
}

// AssignUnowned gives every id of type t that has no owner to account
// owner, each with an owner_set row, all in one transaction, and
// returns the ids it gave, in ascending order. Ids that have an owner
// are left as they are, so running it again assigns nothing.
func AssignUnowned(ctx context.Context, db *sql.DB, t ResourceType, ids []int64, owner uint64,
	by Actor) ([]int64, error) {
	if !t.valid() {
		return nil, ErrUnknownType
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Lock what is owned of the type, so a resource created meanwhile
	// gets its owner from its creator or from here, never both.
	rows, err := tx.QueryContext(ctx, "SELECT resource_id FROM web_resource_relations "+
		"WHERE resource_type = ? AND relation = ? FOR UPDATE", string(t), string(Owner))
	if err != nil {
		return nil, err
	}
	owned, err := scanIDs(rows)
	if err != nil {
		return nil, err
	}
	have := map[int64]bool{}
	for _, id := range owned {
		have[id] = true
	}
	given := []int64{}
	for _, id := range sortedUnique(ids) {
		if have[id] {
			continue
		}
		if err := setOwner(ctx, tx, Resource{t, id}, owner, by, "assign-owner"); err != nil {
			return nil, err
		}
		given = append(given, id)
	}
	return given, tx.Commit()
}

func sortedUnique(ids []int64) []int64 {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
