package gormstore

import (
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type DMDeviceKeyRow struct {
	MemberID    string `gorm:"primaryKey"`
	KeyID       string `gorm:"primaryKey"`
	PublicKey   string
	CreatedAt   time.Time
	PublishedBy *string `gorm:"index"`
	DeviceID    *string `gorm:"index"`
	RetiredAt   *time.Time
}

// BeforeCreate mints a key id (keyed off comm_dm_device_key_seq, "dkey"
// prefix) when the caller left one unset — the memory store's IDGen.New
// did the same, and the Postgres store minted it explicitly via a `SELECT
// nextval(...)` before insert rather than a column DEFAULT (the composite
// primary key means the id column has no DEFAULT clause of its own).
func (r *DMDeviceKeyRow) BeforeCreate(tx *gorm.DB) error {
	if r.KeyID == "" {
		id, err := mintID(tx, "dkey", "comm_dm_device_key_seq")
		if err != nil {
			return err
		}
		r.KeyID = id
	}
	return nil
}

// DMDeviceKeyToRow converts a domain DMDeviceKey to its row shape.
func DMDeviceKeyToRow(k models.DMDeviceKey) DMDeviceKeyRow {
	return DMDeviceKeyRow{
		MemberID:    k.ActorID,
		KeyID:       k.KeyID,
		PublicKey:   k.PublicKey,
		CreatedAt:   k.CreatedAt,
		PublishedBy: StringToNullable(k.PublishedBy),
		DeviceID:    StringToNullable(k.DeviceID),
		RetiredAt:   k.RetiredAt,
	}
}

// DMDeviceKeyFromRow converts a row back to the domain DMDeviceKey.
func DMDeviceKeyFromRow(r DMDeviceKeyRow) models.DMDeviceKey {
	return models.DMDeviceKey{
		ActorID:     r.MemberID,
		KeyID:       r.KeyID,
		PublicKey:   r.PublicKey,
		CreatedAt:   r.CreatedAt,
		PublishedBy: NullableToString(r.PublishedBy),
		DeviceID:    NullableToString(r.DeviceID),
		RetiredAt:   r.RetiredAt,
	}
}
