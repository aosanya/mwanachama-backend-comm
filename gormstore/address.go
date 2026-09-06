package gormstore

import (
	"encoding/json"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// AddressRow is the GORM row for an [models.Address]. Unlike this
// package's other rows, it mints no id: the primary key is Hash itself
// (bytea on Postgres), exactly as the gateway's original member_address
// table had it, so there is no BeforeCreate hook here and no sequence in
// [seqNames] for this table.
type AddressRow struct {
	Hash              []byte `gorm:"primaryKey"`
	SaltID            int
	MemberID          string `gorm:"uniqueIndex:comm_member_address_member_index"`
	AddressIndex      int    `gorm:"uniqueIndex:comm_member_address_member_index"`
	CreatedAt         time.Time
	RetiredAt         *time.Time `gorm:"index:comm_member_address_live_idx"`
	ExpiresAt         *time.Time
	ExpiryMode        string
	MessageTTLSeconds *int
	PublicAddress     *string `gorm:"index:comm_member_address_public_idx"`
	DisabledAt        *time.Time
	DisabledMode      string
	Hours             []byte     `gorm:"type:jsonb"`
	ListedAt          *time.Time `gorm:"index:comm_member_address_listed_idx"`
}

// AddressToRow converts a domain Address to its row shape.
func AddressToRow(a models.Address) (AddressRow, error) {
	hours, err := marshalAddressHours(a.Hours)
	if err != nil {
		return AddressRow{}, err
	}
	return AddressRow{
		Hash:              a.Hash,
		SaltID:            a.SaltID,
		MemberID:          a.MemberID,
		AddressIndex:      a.Index,
		CreatedAt:         a.CreatedAt,
		RetiredAt:         a.RetiredAt,
		ExpiresAt:         a.ExpiresAt,
		ExpiryMode:        a.ExpiryMode,
		MessageTTLSeconds: a.MessageTTLSeconds,
		PublicAddress:     StringToNullable(a.PublicAddress),
		DisabledAt:        a.DisabledAt,
		DisabledMode:      a.DisabledMode,
		Hours:             hours,
		ListedAt:          a.ListedAt,
	}, nil
}

// AddressFromRow converts a row back to the domain Address.
//
// **An unparseable hours JSON reads as no schedule**, and the row is still
// returned — the same errs-open reasoning [models.AddressHours.OpenAt]
// documents: an address nobody can be told about, because one JSON field
// moved, is a worse answer than one shown as open while somebody fixes it.
func AddressFromRow(r AddressRow) models.Address {
	a := models.Address{
		MemberID:  r.MemberID,
		Hash:      r.Hash,
		SaltID:    r.SaltID,
		Index:     r.AddressIndex,
		CreatedAt: r.CreatedAt,
		RetiredAt: r.RetiredAt,
		AddressSettings: models.AddressSettings{
			ExpiresAt:         r.ExpiresAt,
			ExpiryMode:        r.ExpiryMode,
			MessageTTLSeconds: r.MessageTTLSeconds,
			PublicAddress:     NullableToString(r.PublicAddress),
			ListedAt:          r.ListedAt,
			DisabledAt:        r.DisabledAt,
			DisabledMode:      r.DisabledMode,
		},
	}
	if len(r.Hours) > 0 {
		var h models.AddressHours
		if err := json.Unmarshal(r.Hours, &h); err == nil {
			a.Hours = &h
		}
	}
	return a
}

// marshalAddressHours renders the schedule for the jsonb column, or nil for
// an address on no schedule — nil rather than the JSON literal `null`, so
// the column is genuinely NULL.
func marshalAddressHours(h *models.AddressHours) ([]byte, error) {
	if h == nil {
		return nil, nil
	}
	return json.Marshal(h)
}

// AddressBlockRow is the GORM row for a [models.AddressBlock]. Composite
// primary key (member_id, address_hash), matching the gateway's original
// member_address_block table — idempotent by construction, since Block is
// meant to be pressed more than once.
type AddressBlockRow struct {
	MemberID  string `gorm:"primaryKey"`
	Hash      []byte `gorm:"primaryKey"`
	CreatedAt time.Time
}

// AddressBlockToRow converts a domain AddressBlock to its row shape.
func AddressBlockToRow(b models.AddressBlock) AddressBlockRow {
	return AddressBlockRow{MemberID: b.MemberID, Hash: b.Hash, CreatedAt: b.CreatedAt}
}
