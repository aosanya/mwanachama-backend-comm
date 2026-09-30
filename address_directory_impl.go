package mwanachamacomm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type AddressDirectoryStore struct {
	store *store
	clock Clock

	actorsTable string
}

const DefaultActorsTable = "member_actors"

func NewAddressDirectoryStore(db *gorm.DB, s *spec.Spec, actorsTable string, clock Clock) (*AddressDirectoryStore, error) {
	if db == nil {
		return nil, fmt.Errorf("NewAddressDirectoryStore: db must not be nil")
	}
	if clock == nil {
		clock = SystemClock
	}
	if actorsTable == "" {
		actorsTable = DefaultActorsTable
	}
	st, err := newStore(db, s, map[string]any{roleAddress: models.Address{}})
	if err != nil {
		return nil, err
	}
	return &AddressDirectoryStore{store: st, clock: clock, actorsTable: actorsTable}, nil
}

func (s *AddressDirectoryStore) Search(ctx context.Context, q models.AddressDirectoryQuery) ([]models.AddressListing, error) {
	q = q.Normalized()
	now := s.clock()

	collate := ""
	if s.store.Query(ctx, roleAddress).Dialector.Name() == "postgres" {
		collate = ` COLLATE "C"`
	}

	query := `
SELECT a.public_address, a.actor_id, m.display_name, a.listed_at, a.disabled_at, a.hours
  FROM ` + s.store.Table(roleAddress) + ` a
  JOIN ` + s.actorsTable + ` m ON m.id = a.actor_id
 WHERE a.listed_at IS NOT NULL AND a.public_address <> ''
   AND a.retired_at IS NULL AND (a.expires_at IS NULL OR a.expires_at > ?)
   AND (? = '' OR a.actor_id <> ?)
   AND (? = '' OR LOWER(m.display_name) LIKE ? OR (? <> '' AND a.public_address LIKE ?))
 ORDER BY a.listed_at DESC, a.public_address` + collate + `
 LIMIT ? OFFSET ?`

	nameFrag := strings.ToLower(q.Search)
	addrFrag := q.AddressSearch()
	rows, err := s.store.Query(ctx, roleAddress).Raw(query,
		stamp(now),
		q.ExcludeActorID, q.ExcludeActorID,
		q.Search, "%"+nameFrag+"%", addrFrag, "%"+addrFrag+"%",
		q.Limit, q.Offset,
	).Rows()
	if err != nil {
		return nil, classify(err)
	}
	defer func() { _ = rows.Close() }()

	out := []models.AddressListing{}
	for rows.Next() {
		var l models.AddressListing
		var disabled flexTime
		var listed flexTime
		var hours []byte
		if err := rows.Scan(&l.Address, &l.ActorID, &l.DisplayName, &listed, &disabled, &hours); err != nil {
			return nil, classify(err)
		}
		if listed.Valid {
			l.ListedAt = listed.Time
		}
		l.Available = addressAvailableNow(disabled, hours, now)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, classify(err)
	}
	return out, nil
}

func addressAvailableNow(disabled flexTime, hours []byte, now time.Time) bool {
	if disabled.Valid {
		return false
	}
	if len(hours) == 0 {
		return true
	}
	var h models.AddressHours
	if err := json.Unmarshal(hours, &h); err != nil {
		return true
	}
	return h.OpenAt(now)
}

var _ models.AddressDirectory = (*AddressDirectoryStore)(nil)
