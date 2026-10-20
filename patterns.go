package mwanachamacomm

import (
	"regexp"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type pattern func(string) bool

var subjectKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

var patterns = map[string]pattern{
	"address":      models.AddressValid,
	"subject_kind": subjectKindPattern.MatchString,
}
