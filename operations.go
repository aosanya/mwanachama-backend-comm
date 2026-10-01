package mwanachamacomm

import (
	_ "embed"
)

//go:embed comm.operations.json
var operationsJSON []byte

func Operations() []byte { return operationsJSON }
