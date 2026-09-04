package gormstore

// G79's third outcome, on GORM — its own file rather than more of
// moderation.go, mirroring the gateway's original split
// (store_postgres_moderation_dismissal.go, [[file-length-limit]]).

import (
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// DismissalRow is the GORM row for a [models.Dismissal]. MessageID is
// unique: one dismissal per message, and "Left standing" is the existence
// of this row.
type DismissalRow struct {
	ID             string `gorm:"primaryKey"`
	MessageID      string `gorm:"uniqueIndex"`
	ChapterID      string `gorm:"index:comm_message_report_dismissal_chapter_idx,priority:1"`
	DismissedBy    string
	ActorRoleClass string
	Reason         string
	Note           *string
	DismissedAt    time.Time `gorm:"index:comm_message_report_dismissal_chapter_idx,priority:2"`
}

func (r *DismissalRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "moddismissal", "comm_message_report_dismissal_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// DismissalToRow converts a domain Dismissal to its row shape.
func DismissalToRow(d models.Dismissal) DismissalRow {
	return DismissalRow{
		ID:             d.ID,
		MessageID:      d.MessageID,
		ChapterID:      d.ChapterID,
		DismissedBy:    d.DismissedBy,
		ActorRoleClass: d.ActorRoleClass,
		Reason:         string(d.Reason),
		Note:           StringToNullable(d.Note),
		DismissedAt:    d.DismissedAt,
	}
}

// DismissalFromRow converts a row back to the domain Dismissal.
func DismissalFromRow(r DismissalRow) models.Dismissal {
	return models.Dismissal{
		ID:             r.ID,
		MessageID:      r.MessageID,
		ChapterID:      r.ChapterID,
		DismissedBy:    r.DismissedBy,
		ActorRoleClass: r.ActorRoleClass,
		Reason:         models.RemovalReason(r.Reason),
		Note:           NullableToString(r.Note),
		DismissedAt:    r.DismissedAt,
	}
}
