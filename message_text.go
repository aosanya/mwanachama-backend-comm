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

const (
	firstPictographRune = 0x1F000
	lastPictographRune  = 0x1FAFF
)

func isPictograph(r rune) bool {
	return r >= firstPictographRune && r <= lastPictographRune
}

func messageDraws(body string) bool {
	for _, r := range body {
		if isPictograph(r) || (!plaintext.IsUnseenOrLineBreaking(r) && unicode.In(r, unicode.L, unicode.N, unicode.P, unicode.S)) {
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
