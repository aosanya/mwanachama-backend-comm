package mwanachamacomm

import (
	"fmt"
	"unicode"

	"github.com/aosanya/mwanachama-backend-shared/plaintext"
)

var messageBodyRoles = map[string]string{
	roleChatMessage:     "body",
	roleSubjectMessage:  "body",
	roleStandingMessage: "body",
}

func messageDraws(body string) bool {
	for _, r := range body {
		if !plaintext.IsUnseenOrLineBreaking(r) && unicode.In(r, unicode.L, unicode.N, unicode.P, unicode.S) {
			return true
		}
	}
	return false
}

func refuseUndrawnMessage(role, field, text string) error {
	if messageBodyRoles[role] != field || text == "" || messageDraws(text) {
		return nil
	}
	return fmt.Errorf("%w: %s is required", invalid(role), field)
}
