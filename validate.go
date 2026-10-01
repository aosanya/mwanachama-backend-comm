package mwanachamacomm

import (
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// invalidFor is the sentinel each role's refusals carry, so a declared rule
// and the Go rule beside it answer a caller the same way.
var invalidFor = map[string]error{
	roleAddress:                models.ErrAddressBadSettings,
	roleAddressBlock:           models.ErrAddressBadSettings,
	roleNotification:           models.ErrNotificationInvalid,
	roleNotificationPreference: models.ErrNotificationInvalid,
}

func invalid(role string) error {
	if err, ok := invalidFor[role]; ok {
		return err
	}
	return ErrInvalidReference
}

// Check applies every rule the spec states — required, an enum's values, and
// a named pattern — without touching a database, so a bulk import can
// validate what it read before it opens a connection.
func Check(s *spec.Spec, role string, v any) error {
	o, ok := s.ByRole(role)
	if !ok {
		return fmt.Errorf("comm: no object fills the role %q", role)
	}
	row, err := encode(o, v)
	if err != nil {
		return err
	}

	for _, f := range o.Fields {
		cell, held := row[f.Name]
		if !held {
			continue
		}
		text, isText := cell.(string)

		if f.Required && (cell == nil || (isText && strings.TrimSpace(text) == "")) {
			return fmt.Errorf("%w: %s is required", invalid(role), f.Name)
		}
		if cell == nil || !isText {
			continue
		}
		if len(f.Values) > 0 && !holds(f.Values, text) {
			return fmt.Errorf("%w: %q is not one of %s's declared values", invalid(role), text, f.Name)
		}
		if f.Matches != "" && text != "" {
			match, known := patterns[f.Matches]
			if !known {
				return fmt.Errorf("comm: %s.%s names the pattern %q, which this module supplies no rule for",
					role, f.Name, f.Matches)
			}
			if !match(text) {
				return fmt.Errorf("%w: %s does not match %s", invalid(role), f.Name, f.Matches)
			}
		}
	}
	return nil
}

func holds(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
