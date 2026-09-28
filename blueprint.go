package mwanachamacomm

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

//go:embed comm.blueprint.json
var blueprintJSON []byte

//go:embed civic.comm.json
var domainJSON []byte

var loadBlueprint = sync.OnceValues(func() (*spec.Blueprint, error) {
	return spec.ParseBlueprint(blueprintJSON)
})

func Blueprint() (*spec.Blueprint, error) { return loadBlueprint() }

func LoadSpec(path string) (*spec.Spec, error) {
	b, err := Blueprint()
	if err != nil {
		return nil, err
	}
	return b.Load(path)
}

func ParseSpec(raw []byte) (*spec.Spec, error) {
	b, err := Blueprint()
	if err != nil {
		return nil, err
	}
	return b.Parse(raw)
}

func SpecFor(instance string) (*spec.Spec, error) {
	var doc map[string]any
	if err := json.Unmarshal(domainJSON, &doc); err != nil {
		return nil, fmt.Errorf("comm spec: %w", err)
	}
	doc["instance"] = instance

	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("comm spec: %w", err)
	}
	return ParseSpec(raw)
}
