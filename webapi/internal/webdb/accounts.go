// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2026 Alviro Iskandar Setiawan <alviro.iskandar@gnuweeb.org>
 */

package webdb

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// The accounts administration's reads and writes of web_users. Who may
// do what is decided by the caller, from the request scope; these only
// keep the table consistent (the last active superadmin, the epoch
// bumps that end sessions on both services).

// Account is a web_users row as the accounts administration shows it.
type Account struct {
	ID                 uint64
	Username           string
	DisplayName        string
	Email              string // "" when none
	EmailVerified      bool   // Email is verified (web-accounts.md section 8)
	Role               string
	Active             bool
	HasPassword        bool
	MustChangePassword bool
	CreatedBy          uint64 // 0 when none
	CreatedAt          time.Time
	LastSignin         *time.Time // the newest login_ok, nil if none
	// AdminOf is the federations an admin administers, Federations
	// those a member is filed in; ascending, nil for none.
	AdminOf     []uint64
	Federations []uint64
	// HidePrivate is an admin's restriction: private chats are hidden
	// from it (false for the other roles).
	HidePrivate bool
	// JoinChat is an admin's grant of Join chat (false for the other
	// roles).
	JoinChat bool
	// HideGroups are the groups hidden from an admin, ascending.
	HideGroups []int64
	// Grants are a member's granted groups by federation (ascending),
	// Abilities its abilities (ascending); nil for none, and for the
	// other roles.
	Grants    map[uint64][]int64
	Abilities []string
}

// AccountAudit is one audited change of an account.
type AccountAudit struct {
	Action string
	By     string // the acting account's username, "" if gone or none
	Detail string
	At     time.Time
}

var (
	// ErrTaken: the username is in use.
	ErrTaken = errors.New("username taken")
	// ErrEmailTaken: the address is another account's.
	ErrEmailTaken = errors.New("email address taken")
	// ErrLastSuperadmin: the change would leave no active superadmin.
	ErrLastSuperadmin = errors.New("last active superadmin")
)

// Times are read as Unix seconds, so the session time zone the
// DATETIME columns were written in is converted by the server.
const accountColumns = "SELECT u.id, u.username, u.display_name, " +
	"COALESCE(u.email, ''), u.email_verified_at IS NOT NULL, u.role, u.is_active, u.password_hash <> '', " +
	"u.must_change_password, COALESCE(u.created_by, 0), " +
	"UNIX_TIMESTAMP(u.created_at), " +
	"(SELECT UNIX_TIMESTAMP(MAX(a.created_at)) FROM web_audit a " +
	"WHERE a.web_user_id = u.id AND a.action = 'login_ok'), u.hide_private, u.join_chat, " +
	adminOfColumn + ", " + memberOfColumn + ", " + hiddenGroupsColumn + ", " + grantsColumn +
	", " + abilitiesColumn +
	"FROM web_users u "

type scanner interface {
	Scan(dest ...any) error
}

func scanAccount(row scanner) (Account, error) {
	var (
		a                         Account
		created                   float64
		signin                    sql.NullFloat64
		adminOf, memberOf, hidden sql.NullString
		grants, abilities         sql.NullString
	)
	err := row.Scan(&a.ID, &a.Username, &a.DisplayName, &a.Email, &a.EmailVerified, &a.Role,
		&a.Active, &a.HasPassword, &a.MustChangePassword, &a.CreatedBy,
		&created, &signin, &a.HidePrivate, &a.JoinChat, &adminOf, &memberOf, &hidden,
		&grants, &abilities)
	if err != nil {
		return Account{}, err
	}
	if a.AdminOf, err = idList(adminOf); err != nil {
		return Account{}, err
	}
	if a.Federations, err = idList(memberOf); err != nil {
		return Account{}, err
	}
	if a.HideGroups, err = groupList(hidden); err != nil {
		return Account{}, err
	}
	if a.Grants, err = grantMap(grants); err != nil {
		return Account{}, err
	}
	if a.Abilities, err = abilityList(abilities); err != nil {
		return Account{}, err
	}
	a.CreatedAt = time.Unix(int64(created), 0).UTC()
	if signin.Valid {
		t := time.Unix(int64(signin.Float64), 0).UTC()
		a.LastSignin = &t
	}
	return a, nil
}

// ListAccounts is every account, by username.
func ListAccounts(ctx context.Context, db *sql.DB) ([]Account, error) {
	rows, err := db.QueryContext(ctx, accountColumns+"ORDER BY u.username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAccount is one account, or ErrNoUser.
func GetAccount(ctx context.Context, db *sql.DB, id uint64) (Account, error) {
	a, err := scanAccount(db.QueryRowContext(ctx, accountColumns+"WHERE u.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNoUser
	}
	return a, err
}

// ActiveSuperadmins counts the active superadmins.
func ActiveSuperadmins(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM web_users WHERE role = 'superadmin' AND is_active = 1").Scan(&n)
	return n, err
}

// AccountAudits is the newest audited changes of an account, newest
// first.
func AccountAudits(ctx context.Context, db *sql.DB, id uint64, limit int) ([]AccountAudit, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT a.action, COALESCE(u.username, ''), COALESCE(a.detail, ''), "+
			"UNIX_TIMESTAMP(a.created_at) FROM web_audit a "+
			"LEFT JOIN web_users u ON u.id = a.web_user_id "+
			"WHERE a.target_user_id = ? ORDER BY a.id DESC LIMIT ?", id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountAudit
	for rows.Next() {
		var e AccountAudit
		var at float64
		if err := rows.Scan(&e.Action, &e.By, &e.Detail, &at); err != nil {
			return nil, err
		}
		e.At = time.Unix(int64(at), 0).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// NewAccount is what CreateAccount stores.
type NewAccount struct {
	Username    string
	DisplayName string
	Email       string // "" for none
	Role        string
	// PasswordHash is the first password's, typed by the creator or
	// generated; the account must change it at its first sign-in. ""
	// creates it without a password, to be set from the invitation
	// mailed to Email.
	PasswordHash string
	CreatedBy    uint64
	// Federations files a member in these federations; they must
	// exist (else ErrNoFederation).
	Federations []uint64
	// Grants are the member's granted groups in some of Federations
	// (checked by the caller to be those federations' groups).
	Grants GrantChange
}

// duplicate tells which unique key a failed insert or update hit.
func duplicate(err error) error {
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != 1062 {
		return err
	}
	if strings.Contains(me.Message, "uq_web_users_email") {
		return ErrEmailTaken
	}
	return ErrTaken
}

// nullable stores "" as NULL, so several accounts may have no address.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// CreateAccount inserts an account, flagged to change its first
// password at its first sign-in, or without a password for an
// invitation, and audits it in the same transaction.
func CreateAccount(ctx context.Context, db *sql.DB, n NewAccount, ip string) (uint64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		"INSERT INTO web_users (username, display_name, email, password_hash, "+
			"role, is_active, must_change_password, created_by) "+
			"VALUES (?, ?, ?, ?, ?, 1, ?, ?)",
		n.Username, n.DisplayName, nullable(n.Email), n.PasswordHash, n.Role,
		n.PasswordHash != "", n.CreatedBy)
	if err != nil {
		return 0, duplicate(err)
	}
	id64, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	id := uint64(id64)
	detail := n.Username + " role=" + n.Role
	if len(n.Federations) > 0 {
		if _, _, err := setAccountFederations(ctx, tx, "web_federation_members", id,
			FederationChange{Set: n.Federations, WithinAll: true}); err != nil {
			return 0, err
		}
		detail += " federations=" + idsText(uniq(n.Federations))
		more, err := setGrants(ctx, tx, id, n.Grants)
		if err != nil {
			return 0, err
		}
		if len(more) > 0 {
			detail += "; " + strings.Join(more, "; ")
		}
	}
	if err := AuditAccount(ctx, tx, n.CreatedBy, id, "account_create", ip, detail); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// AccountChange is a change to one account; nil fields stay.
type AccountChange struct {
	DisplayName *string
	// Email changes the address, unverified; "" removes it.
	Email  *string
	Role   *string
	Active *bool
	// PasswordHash sets a temporary password, which the account must
	// change at its next sign-in.
	PasswordHash *string
	// ClearPassword removes the password, so the account signs in
	// again only once it has set a new one from a link.
	ClearPassword bool
	// EndSessions bumps session_epoch, ending every session of the
	// account on both services. A role change, a disable and a new
	// password do it as well.
	EndSessions bool
	// AdminOf sets the federations an admin administers, Federations
	// those a member is filed in (ErrNoFederation for one that does
	// not exist). Whatever the account's role ends as, the rows of the
	// other kind go: a member made admin leaves its federations, an
	// admin made anything else stops administering.
	AdminOf     *FederationChange
	Federations *FederationChange
	// HidePrivate sets an admin's restriction on private chats. It is
	// cleared whenever the account ends as another role.
	HidePrivate *bool
	// JoinChat grants or withdraws an admin's Join chat. It is cleared
	// whenever the account ends as another role.
	JoinChat *bool
	// HideGroups replaces the groups hidden from an admin; they are
	// dropped whenever the account ends as another role.
	HideGroups *[]int64
	// Grants replaces a member's granted groups in the federations it
	// names (after Federations is applied); Abilities replaces its
	// abilities. Both are dropped whenever the account ends as another
	// role (the grants with its filings).
	Grants    *GrantChange
	Abilities *[]string
}

// UpdateAccount applies a change and audits it with the actor, in one
// transaction. It refuses, with ErrLastSuperadmin, a change that
// demotes or disables the last active superadmin: the active
// superadmins are locked first, so two such changes made at once
// cannot both pass. It reports whether anything changed; nothing is
// audited when nothing did.
func UpdateAccount(ctx context.Context, db *sql.DB, actor, id uint64, c AccountChange,
	action, ip string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var supers []uint64
	rows, err := tx.QueryContext(ctx, "SELECT id FROM web_users "+
		"WHERE role = 'superadmin' AND is_active = 1 FOR UPDATE")
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var s uint64
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return false, err
		}
		supers = append(supers, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}

	var cur struct {
		name, email, role             string
		active, hidePrivate, joinChat bool
	}
	err = tx.QueryRowContext(ctx, "SELECT display_name, COALESCE(email, ''), role, "+
		"is_active, hide_private, join_chat FROM web_users WHERE id = ? FOR UPDATE", id).
		Scan(&cur.name, &cur.email, &cur.role, &cur.active, &cur.hidePrivate, &cur.joinChat)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNoUser
	}
	if err != nil {
		return false, err
	}

	demoted := c.Role != nil && *c.Role != "superadmin"
	disabled := c.Active != nil && !*c.Active
	if cur.role == "superadmin" && cur.active && (demoted || disabled) &&
		len(supers) == 1 && supers[0] == id {
		return false, ErrLastSuperadmin
	}

	var (
		sets   []string
		args   []any
		detail []string
		bump   = c.EndSessions
	)
	if c.DisplayName != nil && *c.DisplayName != cur.name {
		sets, args = append(sets, "display_name = ?"), append(args, *c.DisplayName)
		detail = append(detail, "name")
	}
	if c.Email != nil && *c.Email != cur.email {
		sets = append(sets, "email = ?", "email_verified_at = NULL")
		args = append(args, nullable(*c.Email))
		detail = append(detail, "email")
	}
	if c.Role != nil && *c.Role != cur.role {
		sets, args = append(sets, "role = ?"), append(args, *c.Role)
		detail = append(detail, "role "+cur.role+"->"+*c.Role)
		bump = true
	}
	if c.Active != nil && *c.Active != cur.active {
		sets, args = append(sets, "is_active = ?"), append(args, *c.Active)
		bump = bump || !*c.Active
	}
	// Only an admin can be restricted: the restriction goes with a
	// role change away from admin.
	hide := cur.hidePrivate
	if c.HidePrivate != nil {
		hide = *c.HidePrivate
	}
	endRole := cur.role
	if c.Role != nil {
		endRole = *c.Role
	}
	if endRole != "admin" {
		hide = false
	}
	if hide != cur.hidePrivate {
		sets, args = append(sets, "hide_private = ?"), append(args, hide)
		detail = append(detail, map[bool]string{true: "hide_private on", false: "hide_private off"}[hide])
	}
	join := cur.joinChat
	if c.JoinChat != nil {
		join = *c.JoinChat
	}
	if endRole != "admin" {
		join = false
	}
	if join != cur.joinChat {
		sets, args = append(sets, "join_chat = ?"), append(args, join)
		detail = append(detail, map[bool]string{true: "join_chat on", false: "join_chat off"}[join])
	}
	if c.HideGroups != nil || endRole != "admin" {
		var want []int64
		if c.HideGroups != nil && endRole == "admin" {
			want = slices.Compact(slices.Sorted(slices.Values(*c.HideGroups)))
		}
		before, err := setHiddenGroups(ctx, tx, id, want)
		if err != nil {
			return false, err
		}
		if !slices.Equal(before, want) {
			detail = append(detail, "hide_groups "+groupsText(before)+"->"+groupsText(want))
		}
	}
	if c.PasswordHash != nil {
		sets, args = append(sets, "password_hash = ?", "must_change_password = 1"),
			append(args, *c.PasswordHash)
		bump = true
	} else if c.ClearPassword {
		sets = append(sets, "password_hash = ''", "must_change_password = 0")
		bump = true
	}
	if bump {
		sets = append(sets, "session_epoch = session_epoch + 1")
	}
	if len(sets) > 0 {
		args = append(args, id)
		if _, err := tx.ExecContext(ctx, "UPDATE web_users SET "+strings.Join(sets, ", ")+
			" WHERE id = ?", args...); err != nil {
			return false, duplicate(err)
		}
	}

	role := cur.role
	if c.Role != nil {
		role = *c.Role
	}
	for _, part := range []struct {
		table, what, role string
		c                 *FederationChange
	}{
		{"web_federation_admins", "admin_of", "admin", c.AdminOf},
		{"web_federation_members", "federations", "member", c.Federations},
	} {
		fc := part.c
		if role != part.role {
			// Not of this kind (any more): its rows go.
			fc = &FederationChange{WithinAll: true}
		}
		if fc == nil {
			continue
		}
		before, after, err := setAccountFederations(ctx, tx, part.table, id, *fc)
		if err != nil {
			return false, err
		}
		if !slices.Equal(before, after) {
			detail = append(detail, part.what+" "+idsText(before)+"->"+idsText(after))
		}
	}
	if c.Grants != nil && role == "member" {
		more, err := setGrants(ctx, tx, id, *c.Grants)
		if err != nil {
			return false, err
		}
		detail = append(detail, more...)
	}
	if c.Abilities != nil || role != "member" {
		var want []string
		if c.Abilities != nil && role == "member" {
			want = *c.Abilities
		}
		more, err := setAbilities(ctx, tx, id, want)
		if err != nil {
			return false, err
		}
		if more != "" {
			detail = append(detail, more)
		}
	}
	if len(sets) == 0 && len(detail) == 0 {
		return false, nil
	}
	if err := AuditAccount(ctx, tx, actor, id, action, ip, strings.Join(detail, "; ")); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// setHiddenGroups replaces an account's hidden groups with want (sorted)
// and answers the ones it had.
func setHiddenGroups(ctx context.Context, tx *sql.Tx, id uint64, want []int64) ([]int64, error) {
	var before sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT JSON_ARRAYAGG(group_id) FROM web_admin_hidden_groups "+
		"WHERE web_user_id = ?", id).Scan(&before); err != nil {
		return nil, err
	}
	had, err := groupList(before)
	if err != nil {
		return nil, err
	}
	if slices.Equal(had, want) {
		return had, nil
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM web_admin_hidden_groups WHERE web_user_id = ?", id); err != nil {
		return nil, err
	}
	for _, g := range want {
		if _, err := tx.ExecContext(ctx, "INSERT INTO web_admin_hidden_groups (web_user_id, group_id) "+
			"VALUES (?, ?)", id, g); err != nil {
			return nil, err
		}
	}
	return had, nil
}

// groupsText is a list of group ids for the audit, "none" for none.
func groupsText(ids []int64) string {
	if len(ids) == 0 {
		return "none"
	}
	return JoinGroupIDs(ids)
}
