// Package gormstore holds every GORM-specific piece of this repo: row
// structs, their conversion to/from the domain types in
// mwanachama-backend-comm/models, and table migration. Nothing outside this
// package (and the root mwanachama-backend-comm package's *_impl.go files,
// which call it) needs to know GORM exists — mirrors
// mwanachama-backend-actor/gormstore's split.
package gormstore

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TableNames struct {
	ChatThreads  string
	ChatMessages string

	DMThreads      string
	DMParticipants string
	DMMessages     string
	DMDeviceKeys   string
	DMReactions    string

	Reports    string
	Removals   string
	Disputes   string
	Dismissals string

	Addresses     string
	AddressBlocks string

	Notifications           string
	NotificationPreferences string
}

// DefaultTableNames returns comm's fifteen production table names — the
// original eleven are the names migration 000065_extract_comm_tables.up.sql
// renamed the gateway's original chat_thread/dm_thread/message_report/etc.
// tables to; the four address/notification tables are new, minted under the
// same comm_ prefix by this package's own [Migrate].
func DefaultTableNames() TableNames {
	return TableNames{
		ChatThreads:  "comm_chat_thread",
		ChatMessages: "comm_chat_message",

		DMThreads:      "comm_dm_thread",
		DMParticipants: "comm_dm_participant",
		DMMessages:     "comm_dm_message",
		DMDeviceKeys:   "comm_dm_device_key",
		DMReactions:    "comm_dm_message_reaction",

		Reports:    "comm_message_report",
		Removals:   "comm_message_removal",
		Disputes:   "comm_removal_dispute",
		Dismissals: "comm_message_report_dismissal",

		Addresses:     "comm_member_address",
		AddressBlocks: "comm_member_address_block",

		Notifications:           "comm_notification",
		NotificationPreferences: "comm_notification_preference",
	}
}

// Migrate creates or updates the fifteen tables t names, via GORM's
// AutoMigrate scoped to each table name in turn, plus the Postgres
// SEQUENCEs the id-minting BeforeCreate hooks below read from (see
// mintID) and the few constraints/indexes AutoMigrate cannot express from
// a Go struct tag alone. Callers run this once at startup (or in test
// setup) before constructing a store with the same db and t.
func Migrate(db *gorm.DB, t TableNames) error {
	if db.Dialector.Name() == "postgres" {
		if err := createSequences(db); err != nil {
			return err
		}
	}
	migrations := []struct {
		table string
		row   any
	}{
		{t.ChatThreads, &ChatThreadRow{}},
		{t.ChatMessages, &ChatMessageRow{}},
		{t.DMThreads, &DMThreadRow{}},
		{t.DMParticipants, &DMParticipantRow{}},
		{t.DMMessages, &DMMessageRow{}},
		{t.DMDeviceKeys, &DMDeviceKeyRow{}},
		{t.DMReactions, &DMReactionRow{}},
		{t.Reports, &ReportRow{}},
		{t.Removals, &RemovalRow{}},
		{t.Disputes, &DisputeRow{}},
		{t.Dismissals, &DismissalRow{}},
		{t.Addresses, &AddressRow{}},
		{t.AddressBlocks, &AddressBlockRow{}},
		{t.Notifications, &NotificationRow{}},
		{t.NotificationPreferences, &NotificationPreferenceRow{}},
	}
	for _, m := range migrations {
		if err := db.Table(m.table).AutoMigrate(m.row); err != nil {
			return fmt.Errorf("gormstore.Migrate: %s: %w", m.table, err)
		}
	}
	if err := syncNotificationCapIndexes(db, t.Notifications); err != nil {
		return err
	}
	return nil
}

func syncNotificationCapIndexes(db *gorm.DB, table string) error {
	stmts := []string{
		fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS %s_one_reminder_per_subject ON %s (member_id, subject_id) WHERE event = 'survey_reminder'`,
			table, table,
		),
		fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS %s_one_nudge_per_subject ON %s (chapter_id, subject_id) WHERE event = 'survey_nudge'`,
			table, table,
		),
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("syncNotificationCapIndexes: %w", err)
		}
	}
	return nil
}

// seqNames is every Postgres SEQUENCE an id-minting BeforeCreate hook reads
// from (see mintID) — the original nine sequences schema.sql used to
// declare, plus one each for notification and notification_preference
// (address rows mint no id at all — see AddressRow). Not table-name-scoped:
// comm has no multi-instance mount today (unlike actor's Actors/Groups), so
// a fixed name matches what already exists in the gateway's live database.
var seqNames = []string{
	"comm_chat_thread_seq",
	"comm_chat_message_seq",
	"comm_dm_thread_seq",
	"comm_dm_message_seq",
	"comm_dm_device_key_seq",
	"comm_message_report_seq",
	"comm_message_removal_seq",
	"comm_removal_dispute_seq",
	"comm_message_report_dismissal_seq",
	"comm_notification_seq",
	"comm_notification_preference_seq",
}

func createSequences(db *gorm.DB) error {
	for _, seq := range seqNames {
		if err := db.Exec("CREATE SEQUENCE IF NOT EXISTS " + seq).Error; err != nil {
			return fmt.Errorf("gormstore.Migrate: %s: %w", seq, err)
		}
	}
	return nil
}

// mintID produces a human-readable, prefix-and-number id the way the
// production Postgres schema always has (e.g. "msg-1042"), by reading the
// next value of a server-side SEQUENCE on Postgres.
//
// sqlite (this repo's fast-test dialect only — see the root package's
// *_test.go) has no SEQUENCE, so it falls back to a uuid-suffixed id with
// the same prefix. Every behavioural assertion this repo's tests make
// (non-empty, unique, stable sort order via CreatedAt-then-id) holds under
// either dialect; only the exact digits after the prefix differ, which
// nothing depends on.
func mintID(tx *gorm.DB, prefix, seq string) (string, error) {
	if tx.Dialector.Name() == "postgres" {
		var n int64
		if err := tx.Raw("SELECT nextval(?::regclass)", seq).Scan(&n).Error; err != nil {
			return "", fmt.Errorf("mintID: %s: %w", seq, err)
		}
		return fmt.Sprintf("%s-%d", prefix, n), nil
	}
	return prefix + "-" + uuid.NewString(), nil
}
