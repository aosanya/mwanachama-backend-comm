package mwanachamacomm

import (
	"regexp"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

type pattern func(string) bool

var subjectKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

var attachmentIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

var patterns = map[string]pattern{
	"address":       models.AddressValid,
	"subject_kind":  subjectKindPattern.MatchString,
	"attachment_id": attachmentIDPattern.MatchString,
}
