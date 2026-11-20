// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package server

import (
	"context"
)

// The accounts and federations pages' loaders. Both pages are the
// viewer's only when their menu keys are (Scope.Page); for anyone else
// they do not exist (not_found). Their federation data comes from
// federationprops.go (accountsEditable, accountsFederations,
// federationsProps), the one source of it.

type accountsProps struct {
	Accounts       []accountJSON        `json:"accounts"`
	Federations    []fedCountedJSON     `json:"federations"`
	AllFederations []fedJSON            `json:"all_federations"`
	Editable       accountsEditableJSON `json:"editable"`
	// Mail is whether mail is configured, so the page offers
	// set-password links only when they can be sent.
	Mail bool `json:"mail"`
}

func loadAccounts(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "accounts"); err != nil {
		return nil, err
	}
	s := pr.s
	accounts, err := s.managedAccounts(ctx, pr.scope, "")
	if err != nil {
		return nil, err
	}
	managed, all, err := s.accountsFederations(ctx, pr.scope)
	if err != nil {
		return nil, err
	}
	return accountsProps{Accounts: accounts, Federations: managed, AllFederations: all,
		Editable: accountsEditable(), Mail: s.mailOn()}, nil
}

func loadFederations(ctx context.Context, pr *pageRequest) (any, error) {
	if _, err := pageOf(pr, "federations"); err != nil {
		return nil, err
	}
	return pr.s.federationsProps(ctx, pr.scope)
}
