package mwanachamacomm

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func Provision(db *gorm.DB, s *spec.Spec) error {
	if err := adoptLegacy(db, s); err != nil {
		return err
	}
	if err := spec.Migrate(db, s); err != nil {
		return err
	}
	return syncNotificationCaps(db, s)
}

// legacyTables is what each role's table was called before the module
// segment existed. comm's pre-spec names carry no instance segment at all,
// so spec.LegacyTables cannot derive them the way it does for a module whose
// old names were already instance-prefixed.
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

var commLegacy = spec.Legacy{
	Columns: map[string]string{
		"chapter_id":        "structure_id",
		"member_id":         "actor_id",
		"review_chapter_id": "review_structure_id",
		"seat_chapter_id":   "seat_structure_id",
		"author_member_id":  "author_actor_id",
	},
	IndexPrefixes: []string{"comm_", "idx_"},
}

func adoptLegacy(db *gorm.DB, s *spec.Spec) error {
	m := db.Migrator()
	for _, o := range s.Objects {
		declared := s.TableFor(o)

		if name, ok := legacyTables[o.Role]; ok && name != declared && m.HasTable(name) {
			if m.HasTable(declared) {
				stranded, err := spec.RowCount(db, name)
				if err != nil {
					return err
				}
				if stranded > 0 {
					return fmt.Errorf("comm: %s and %s both exist and %s still holds %d row(s), which no read would ever reach again",
						name, declared, name, stranded)
				}
			} else {
				if err := db.Exec(fmt.Sprintf("alter table %s rename to %s", name, declared)).Error; err != nil {
					return fmt.Errorf("comm: rename %s to %s: %w", name, declared, err)
				}
				if err := spec.DropIndexes(db, declared, commLegacy); err != nil {
					return err
				}
			}
		}

		if !m.HasTable(declared) {
			continue
		}
		if err := renameColumns(db, declared, o); err != nil {
			return err
		}
	}
	return nil
}

func renameColumns(db *gorm.DB, table string, o spec.Object) error {
	for from, to := range commLegacy.Columns {
		if !spec.DeclaresColumn(o, to) {
			continue
		}
		held, err := spec.HasColumn(db, table, from)
		if err != nil {
			return err
		}
		already, err := spec.HasColumn(db, table, to)
		if err != nil {
			return err
		}
		if !held || already {
			continue
		}
		if err := db.Exec(fmt.Sprintf("alter table %s rename column %s to %s", table, from, to)).Error; err != nil {
			return fmt.Errorf("comm: rename %s.%s to %s: %w", table, from, to, err)
		}
	}
	return nil
}

// syncNotificationCaps applies the two caps the declaration has no way to
// carry: one reminder per actor per subject, and one nudge per structure per
// subject. Both are partial unique indexes over a condition that is not
// `deleted`, which is the only one the format names.
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
