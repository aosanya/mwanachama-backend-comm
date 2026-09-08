package gormstore

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type DMMessageRow struct {
	ID                string `gorm:"primaryKey"`
	ThreadID          string `gorm:"index:comm_dm_message_thread_idx,priority:1"`
	SenderID          string
	SenderDeviceKeyID string
	PayloadCiphertext string
	PerRecipientKeys  datatypes.JSON
	CreatedAt         time.Time `gorm:"index:comm_dm_message_thread_idx,priority:2"`
}

func (r *DMMessageRow) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		id, err := mintID(tx, "dmmsg", "comm_dm_message_seq")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

// DMMessageToRow converts a domain DMMessage to its row shape.
func DMMessageToRow(m models.DMMessage) (DMMessageRow, error) {
	perRecipient := m.PerRecipientKeys
	if perRecipient == nil {
		perRecipient = map[string]string{}
	}
	raw, err := json.Marshal(perRecipient)
	if err != nil {
		return DMMessageRow{}, err
	}
	return DMMessageRow{
		ID:                m.ID,
		ThreadID:          m.ThreadID,
		SenderID:          m.SenderID,
		SenderDeviceKeyID: m.SenderDeviceKeyID,
		PayloadCiphertext: m.PayloadCiphertext,
		PerRecipientKeys:  datatypes.JSON(raw),
		CreatedAt:         m.CreatedAt,
	}, nil
}

// DMMessageFromRow converts a row back to the domain DMMessage.
func DMMessageFromRow(r DMMessageRow) models.DMMessage {
	out := models.DMMessage{
		ID:                r.ID,
		ThreadID:          r.ThreadID,
		SenderID:          r.SenderID,
		SenderDeviceKeyID: r.SenderDeviceKeyID,
		PayloadCiphertext: r.PayloadCiphertext,
		CreatedAt:         r.CreatedAt,
	}
	if len(r.PerRecipientKeys) > 0 {
		_ = json.Unmarshal(r.PerRecipientKeys, &out.PerRecipientKeys)
	}
	if out.PerRecipientKeys == nil {
		out.PerRecipientKeys = map[string]string{}
	}
	return out
}

type DMReactionRow struct {
	MessageID string `gorm:"primaryKey;index:comm_dm_message_reaction_message_idx"`
	MemberID  string `gorm:"primaryKey"`
	Emoji     string
	CreatedAt time.Time
}

// DMReactionToRow converts a domain DMReaction to its row shape.
func DMReactionToRow(r models.DMReaction) DMReactionRow {
	return DMReactionRow{MessageID: r.MessageID, MemberID: r.ActorID, Emoji: r.Emoji, CreatedAt: r.CreatedAt}
}

// DMReactionFromRow converts a row back to the domain DMReaction.
func DMReactionFromRow(r DMReactionRow) models.DMReaction {
	return models.DMReaction{MessageID: r.MessageID, ActorID: r.MemberID, Emoji: r.Emoji, CreatedAt: r.CreatedAt}
}
