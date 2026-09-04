package gormstore

import (
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// ReportRow is the GORM row for a [models.Report]. Unique(message_id,
// reported_by) — message-report.md's "one open report per (message,
// reporter)".
type ReportRow struct {
	ID                string `gorm:"primaryKey"`
	MessageID         string `gorm:"uniqueIndex:comm_message_report_msg_reporter;index:comm_message_report_queue_idx,priority:2"`
	ChapterID         string `gorm:"index:comm_message_report_queue_idx,priority:1"`
	ReportedBy        string `gorm:"uniqueIndex:comm_message_report_msg_reporter"`
	ReporterRoleClass string
	Reason            string
	Note              *string
	Excerpt           string
	ReportedAt        time.Time
}

func (r *ReportRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "modreport", "comm_message_report_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// ReportToRow converts a domain Report to its row shape.
func ReportToRow(r models.Report) ReportRow {
	return ReportRow{
		ID:                r.ID,
		MessageID:         r.MessageID,
		ChapterID:         r.ChapterID,
		ReportedBy:        r.ReportedBy,
		ReporterRoleClass: r.ReporterRoleClass,
		Reason:            string(r.Reason),
		Note:              StringToNullable(r.Note),
		Excerpt:           r.Excerpt,
		ReportedAt:        r.ReportedAt,
	}
}

// ReportFromRow converts a row back to the domain Report.
func ReportFromRow(r ReportRow) models.Report {
	return models.Report{
		ID:                r.ID,
		MessageID:         r.MessageID,
		ChapterID:         r.ChapterID,
		ReportedBy:        r.ReportedBy,
		ReporterRoleClass: r.ReporterRoleClass,
		Reason:            models.ReportReason(r.Reason),
		Note:              NullableToString(r.Note),
		Excerpt:           r.Excerpt,
		ReportedAt:        r.ReportedAt,
	}
}

// RemovalRow is the GORM row for a [models.Removal]. MessageID is unique:
// one removal per message, and "Removed" is the existence of this row.
type RemovalRow struct {
	ID             string `gorm:"primaryKey"`
	MessageID      string `gorm:"uniqueIndex"`
	ChapterID      string
	RemovedBy      string
	ActorRoleClass string
	Reason         string
	RemovedAt      time.Time
}

func (r *RemovalRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "modremoval", "comm_message_removal_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// RemovalToRow converts a domain Removal to its row shape.
func RemovalToRow(r models.Removal) RemovalRow {
	return RemovalRow{
		ID:             r.ID,
		MessageID:      r.MessageID,
		ChapterID:      r.ChapterID,
		RemovedBy:      r.RemovedBy,
		ActorRoleClass: r.ActorRoleClass,
		Reason:         string(r.Reason),
		RemovedAt:      r.RemovedAt,
	}
}

// RemovalFromRow converts a row back to the domain Removal.
func RemovalFromRow(r RemovalRow) models.Removal {
	return models.Removal{
		ID:             r.ID,
		MessageID:      r.MessageID,
		ChapterID:      r.ChapterID,
		RemovedBy:      r.RemovedBy,
		ActorRoleClass: r.ActorRoleClass,
		Reason:         models.RemovalReason(r.Reason),
		RemovedAt:      r.RemovedAt,
	}
}

// DisputeRow is the GORM row for a [models.Dispute]. RemovalID is unique:
// one dispute per removal.
type DisputeRow struct {
	ID              string `gorm:"primaryKey"`
	RemovalID       string `gorm:"uniqueIndex"`
	RaisedBy        string
	Statement       string
	ReviewChapterID string
	HeldSince       time.Time
	RaisedAt        time.Time
	State           string
	DecidedBy       *string
	DecidedAt       *time.Time
}

func (r *DisputeRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "moddispute", "comm_removal_dispute_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// DisputeToRow converts a domain Dispute to its row shape.
func DisputeToRow(d models.Dispute) DisputeRow {
	return DisputeRow{
		ID:              d.ID,
		RemovalID:       d.RemovalID,
		RaisedBy:        d.RaisedBy,
		Statement:       d.Statement,
		ReviewChapterID: d.ReviewChapterID,
		HeldSince:       d.HeldSince,
		RaisedAt:        d.RaisedAt,
		State:           string(d.State),
		DecidedBy:       StringToNullable(d.DecidedBy),
		DecidedAt:       d.DecidedAt,
	}
}

// DisputeFromRow converts a row back to the domain Dispute.
func DisputeFromRow(r DisputeRow) models.Dispute {
	return models.Dispute{
		ID:              r.ID,
		RemovalID:       r.RemovalID,
		RaisedBy:        r.RaisedBy,
		Statement:       r.Statement,
		ReviewChapterID: r.ReviewChapterID,
		HeldSince:       r.HeldSince,
		RaisedAt:        r.RaisedAt,
		State:           models.DisputeState(r.State),
		DecidedBy:       NullableToString(r.DecidedBy),
		DecidedAt:       r.DecidedAt,
	}
}
