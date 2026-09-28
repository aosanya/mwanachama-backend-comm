package mwanachamacomm

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func Provision(db *gorm.DB, s *spec.Spec) error {
	if err := renameLegacyTables(db, s); err != nil {
		return err
	}
	if err := renameLegacyColumns(db, s); err != nil {
		return err
	}
	if err := spec.Migrate(db, s); err != nil {
		return err
	}
	return syncNotificationCaps(db, s)
}

// legacyTables maps each role to what its table was called before the module
// segment existed. comm's pre-spec names carry no instance segment at all —
// they are the literal `comm_` prefix migration 000065 renamed the gateway's
// original bare names to — so this cannot be derived from the spec the way
// forms derives its own, and is written out instead.
var legacyTables = map[string]string{
	roleChatThread:             "comm_chat_thread",
	roleChatMessage:            "comm_chat_message",
	roleDMThread:               "comm_dm_thread",
	roleDMParticipant:          "comm_dm_participant",
	roleDMMessage:              "comm_dm_message",
	roleDMReaction:             "comm_dm_message_reaction",
	roleDMDeviceKey:            "comm_dm_device_key",
	roleReport:                 "comm_message_report",
	roleRemoval:                "comm_message_removal",
	roleDismissal:              "comm_message_report_dismissal",
	roleDispute:                "comm_removal_dispute",
	roleAddress:                "comm_member_address",
	roleAddressBlock:           "comm_member_address_block",
	roleNotification:           "comm_notification",
	roleNotificationPreference: "comm_notification_preference",
}

// legacyColumns is every column the declared field names move. The row structs
// said chapter and member because the gateway's original tables did; the domain
// types have said structure and actor since, and gormstore's ToRow/FromRow
// pairs did the translating. The store now joins a declared column to a Go
// field by name, so a column left behind would read back empty rather than
// fail — which is why this runs whether or not the table itself was renamed.
var legacyColumns = map[string]string{
	"chapter_id":        "structure_id",
	"member_id":         "actor_id",
	"review_chapter_id": "review_structure_id",
	"seat_chapter_id":   "seat_structure_id",
	"author_member_id":  "author_actor_id",
}

func renameLegacyTables(db *gorm.DB, s *spec.Spec) error {
	m := db.Migrator()
	for _, o := range s.Objects {
		legacy, ok := legacyTables[o.Role]
		if !ok {
			continue
		}
		declared := s.TableFor(o)
		if legacy == declared || !m.HasTable(legacy) {
			continue
		}
		if m.HasTable(declared) {
			stranded, err := rowCount(db, legacy)
			if err != nil {
				return err
			}
			if stranded > 0 {
				return fmt.Errorf("comm: %s and %s both exist and %s still holds %d row(s), which no read would ever reach again",
					legacy, declared, legacy, stranded)
			}
			continue
		}
		if err := db.Exec(fmt.Sprintf("alter table %s rename to %s", legacy, declared)).Error; err != nil {
			return fmt.Errorf("comm: rename %s to %s: %w", legacy, declared, err)
		}
		if err := dropLegacyIndexes(db, declared); err != nil {
			return err
		}
	}
	return nil
}

func renameLegacyColumns(db *gorm.DB, s *spec.Spec) error {
	for _, o := range s.Objects {
		table := s.TableFor(o)
		if !db.Migrator().HasTable(table) {
			continue
		}
		for legacy, declared := range legacyColumns {
			if !declaresColumn(o, declared) {
				continue
			}
			present, err := hasColumn(db, table, legacy)
			if err != nil {
				return err
			}
			if !present {
				continue
			}
			already, err := hasColumn(db, table, declared)
			if err != nil {
				return err
			}
			if already {
				continue
			}
			stmt := fmt.Sprintf("alter table %s rename column %s to %s", table, legacy, declared)
			if err := db.Exec(stmt).Error; err != nil {
				return fmt.Errorf("comm: rename %s.%s to %s: %w", table, legacy, declared, err)
			}
		}
	}
	return nil
}

// syncNotificationCaps applies the two caps the declaration has no way to
// carry: one reminder per actor per subject, and one nudge per structure per
// subject. Both are partial unique indexes over a condition that is not
// `deleted`, which is the only one the format names. Partial-index syntax is
// identical on Postgres and SQLite, so one statement per index covers both.
func syncNotificationCaps(db *gorm.DB, s *spec.Spec) error {
	o, ok := s.ByRole(roleNotification)
	if !ok {
		return nil
	}
	table := s.TableFor(o)
	stmts := []string{
		fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS %s_one_reminder_per_subject ON %s (actor_id, subject_id) WHERE event = 'survey_reminder'`,
			table, table,
		),
		fmt.Sprintf(
			`CREATE UNIQUE INDEX IF NOT EXISTS %s_one_nudge_per_subject ON %s (structure_id, subject_id) WHERE event = 'survey_nudge'`,
			table, table,
		),
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("comm: syncNotificationCaps: %w", err)
		}
	}
	return nil
}

func dropLegacyIndexes(db *gorm.DB, table string) error {
	query := `select name from sqlite_master where type = 'index' and tbl_name = ?`
	if db.Dialector.Name() == "postgres" {
		query = `select indexname from pg_indexes where tablename = ?`
	}
	var names []string
	if err := db.Raw(query, table).Scan(&names).Error; err != nil {
		return fmt.Errorf("comm: read indexes of %s: %w", table, err)
	}
	for _, name := range names {
		if len(name) < 5 || name[:5] != "comm_" {
			continue
		}
		if err := db.Exec("drop index if exists " + name).Error; err != nil {
			return fmt.Errorf("comm: drop index %s: %w", name, err)
		}
	}
	return nil
}

func rowCount(db *gorm.DB, table string) (int64, error) {
	var n int64
	if err := db.Table(table).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("comm: count %s: %w", table, err)
	}
	return n, nil
}

func hasColumn(db *gorm.DB, table, column string) (bool, error) {
	query := `select 1 from pragma_table_info(?) where name = ?`
	if db.Dialector.Name() == "postgres" {
		query = `select 1 from information_schema.columns where table_name = ? and column_name = ?`
	}
	var found []int
	if err := db.Raw(query, table, column).Scan(&found).Error; err != nil {
		return false, fmt.Errorf("comm: read columns of %s: %w", table, err)
	}
	return len(found) > 0, nil
}

func declaresColumn(o spec.Object, column string) bool {
	for _, f := range o.Fields {
		if f.Name == column {
			return true
		}
	}
	return false
}
