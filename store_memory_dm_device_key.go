package mwanachamacomm

import (
	"context"
	"time"
)

// RetireDeviceKeysForDevice retires every live key one handset published
// (DEV-1272) and reports how many it retired. Mirrors the Postgres store's
// `UPDATE comm_dm_device_key SET retired_at = … WHERE device_id = $1 AND
// retired_at IS NULL`.
//
// Scoped to the device and not to the member: a member with two handsets
// signs one out and the other must go on receiving. An empty deviceID
// retires nothing and is not an error — a session minted by the phone flow
// carries no device, and treating "" as one would retire every key
// published without a handset.
func (s *DMMemoryStore) RetireDeviceKeysForDevice(_ context.Context, deviceID string, at time.Time) (int, error) {
	if deviceID == "" {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if at.IsZero() {
		at = s.clock()
	}
	n := 0
	for mapKey, k := range s.deviceKeys {
		if k.DeviceID != deviceID || k.RetiredAt != nil {
			continue
		}
		when := at
		k.RetiredAt = &when
		s.deviceKeys[mapKey] = k
		n++
	}
	return n, nil
}

var _ DMRepository = (*DMMemoryStore)(nil)
