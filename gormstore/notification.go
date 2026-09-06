package gormstore

import (
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// NotificationRow is the GORM row for a [models.Notification].
//
// The two caps (one reminder per member per survey, one nudge per chapter
// per survey) are partial unique indexes AutoMigrate cannot express from a
// struct tag alone — see notificationIndexSQL, run after AutoMigrate in
// [Migrate], mirroring mwanachama-backend-actor's
// syncUniqueAttributeIndexes for the same reason.
type NotificationRow struct {
	ID             string `gorm:"primaryKey"`
	MemberID       string `gorm:"index:comm_notification_member_created_idx,priority:1"`
	AuthorMemberID *string
	SeatRoleKindID *string
	SeatChapterID  *string
	Category       string
	Event          string
	SubjectKind    string
	SubjectID      string
	ChapterID      *string
	CreatedAt      time.Time `gorm:"index:comm_notification_member_created_idx,priority:2"`
	ReadAt         *time.Time
}

func (r *NotificationRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "notif", "comm_notification_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// NotificationToRow converts a domain Notification to its row shape.
func NotificationToRow(n models.Notification) NotificationRow {
	return NotificationRow{
		ID:             n.ID,
		MemberID:       n.MemberID,
		AuthorMemberID: StringToNullable(n.AuthorMemberID),
		SeatRoleKindID: StringToNullable(n.SeatRoleKindID),
		SeatChapterID:  StringToNullable(n.SeatChapterID),
		Category:       string(n.Category),
		Event:          string(n.Event),
		SubjectKind:    string(n.SubjectKind),
		SubjectID:      n.SubjectID,
		ChapterID:      StringToNullable(n.ChapterID),
		CreatedAt:      n.CreatedAt,
		ReadAt:         n.ReadAt,
	}
}

// NotificationFromRow converts a row back to the domain Notification.
func NotificationFromRow(r NotificationRow) models.Notification {
	return models.Notification{
		ID:             r.ID,
		MemberID:       r.MemberID,
		AuthorMemberID: NullableToString(r.AuthorMemberID),
		SeatRoleKindID: NullableToString(r.SeatRoleKindID),
		SeatChapterID:  NullableToString(r.SeatChapterID),
		Category:       models.NotificationCategory(r.Category),
		Event:          models.NotificationEvent(r.Event),
		SubjectKind:    models.NotificationSubjectKind(r.SubjectKind),
		SubjectID:      r.SubjectID,
		ChapterID:      NullableToString(r.ChapterID),
		CreatedAt:      r.CreatedAt,
		ReadAt:         r.ReadAt,
	}
}

// NotificationPreferenceRow is the GORM row for a
// [models.NotificationPreference]. Unique(member_id, category) is the
// upsert target SetPreference relies on.
type NotificationPreferenceRow struct {
	ID        string `gorm:"primaryKey"`
	MemberID  string `gorm:"uniqueIndex:comm_notification_preference_member_category"`
	Category  string `gorm:"uniqueIndex:comm_notification_preference_member_category"`
	Muted     bool
	ChangedAt time.Time
}

func (r *NotificationPreferenceRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "notifpref", "comm_notification_preference_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// NotificationPreferenceToRow converts a domain NotificationPreference to
// its row shape.
func NotificationPreferenceToRow(p models.NotificationPreference) NotificationPreferenceRow {
	return NotificationPreferenceRow{
		ID:        p.ID,
		MemberID:  p.MemberID,
		Category:  string(p.Category),
		Muted:     p.Muted,
		ChangedAt: p.ChangedAt,
	}
}

// NotificationPreferenceFromRow converts a row back to the domain
// NotificationPreference.
func NotificationPreferenceFromRow(r NotificationPreferenceRow) models.NotificationPreference {
	return models.NotificationPreference{
		ID:        r.ID,
		MemberID:  r.MemberID,
		Category:  models.NotificationCategory(r.Category),
		Muted:     r.Muted,
		ChangedAt: r.ChangedAt,
	}
}
