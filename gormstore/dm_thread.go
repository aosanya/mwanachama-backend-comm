package gormstore

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// DMThreadRow is the GORM row for a [models.DMThread].
type DMThreadRow struct {
	ID        string `gorm:"primaryKey"`
	Title     *string
	CreatedBy string
	CreatedAt time.Time

	OpenedViaAddressHash  []byte
	OpenedViaAddressOwner *string
	OpenedViaAddressIndex *int
	SentFromAddressOwner  *string
	SentFromAddressIndex  *int
	SentFromAddressSealed datatypes.JSON
	MessageTTLSeconds     *int
}

func (r *DMThreadRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "dm", "comm_dm_thread_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// DMThreadToRow converts a domain DMThread to its row shape.
func DMThreadToRow(t models.DMThread) DMThreadRow {
	var sealed datatypes.JSON
	if len(t.SentFromAddressSealed) > 0 {
		sealed = datatypes.JSON(t.SentFromAddressSealed)
	}
	return DMThreadRow{
		ID:                    t.ID,
		Title:                 StringToNullable(t.Title),
		CreatedBy:             t.CreatedBy,
		CreatedAt:             t.CreatedAt,
		OpenedViaAddressHash:  t.OpenedViaAddressHash,
		OpenedViaAddressOwner: StringToNullable(t.OpenedViaAddressOwner),
		OpenedViaAddressIndex: t.OpenedViaAddressIndex,
		SentFromAddressOwner:  StringToNullable(t.SentFromAddressOwner),
		SentFromAddressIndex:  t.SentFromAddressIndex,
		SentFromAddressSealed: sealed,
		MessageTTLSeconds:     t.MessageTTLSeconds,
	}
}

// DMThreadFromRow converts a row back to the domain DMThread. OpenedViaAddress
// is derived from OpenedViaAddressHash's presence, matching the gateway's own
// "a bare boolean is what the client actually gets" contract.
func DMThreadFromRow(r DMThreadRow) models.DMThread {
	var sealed json.RawMessage
	if len(r.SentFromAddressSealed) > 0 {
		sealed = json.RawMessage(r.SentFromAddressSealed)
	}
	return models.DMThread{
		ID:                    r.ID,
		Title:                 NullableToString(r.Title),
		CreatedBy:             r.CreatedBy,
		CreatedAt:             r.CreatedAt,
		OpenedViaAddressHash:  r.OpenedViaAddressHash,
		OpenedViaAddress:      len(r.OpenedViaAddressHash) > 0,
		OpenedViaAddressOwner: NullableToString(r.OpenedViaAddressOwner),
		OpenedViaAddressIndex: r.OpenedViaAddressIndex,
		SentFromAddressOwner:  NullableToString(r.SentFromAddressOwner),
		SentFromAddressIndex:  r.SentFromAddressIndex,
		SentFromAddressSealed: sealed,
		MessageTTLSeconds:     r.MessageTTLSeconds,
	}
}

type DMParticipantRow struct {
	ThreadID  string `gorm:"primaryKey"`
	MemberID  string `gorm:"primaryKey;index:comm_dm_participant_member_idx,priority:1"`
	State     string `gorm:"index:comm_dm_participant_member_idx,priority:2"`
	IsAdmin   bool
	UpdatedAt time.Time
}

// DMParticipantToRow converts a domain DMParticipant to its row shape.
func DMParticipantToRow(p models.DMParticipant) DMParticipantRow {
	return DMParticipantRow{
		ThreadID:  p.ThreadID,
		MemberID:  p.ActorID,
		State:     string(p.State),
		IsAdmin:   p.IsAdmin,
		UpdatedAt: p.UpdatedAt,
	}
}

// DMParticipantFromRow converts a row back to the domain DMParticipant.
func DMParticipantFromRow(r DMParticipantRow) models.DMParticipant {
	return models.DMParticipant{
		ThreadID:  r.ThreadID,
		ActorID:   r.MemberID,
		State:     models.DMParticipantState(r.State),
		IsAdmin:   r.IsAdmin,
		UpdatedAt: r.UpdatedAt,
	}
}
