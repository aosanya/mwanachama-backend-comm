package mwanachamacomm

import "github.com/aosanya/mwanachama-backend-comm/models"

type pattern func(string) bool

var patterns = map[string]pattern{
	"address": models.AddressValid,
}
