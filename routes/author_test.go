package routes

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

var namesAnotherPersonByDesign = map[string]bool{
	"invite_to_dm_thread":     true,
	"kick_from_dm_thread":     true,
	"promote_dm_participant":  true,
	"reenable_dm_participant": true,
}

func TestEveryCommAuthorIsTheCaller(t *testing.T) {
	spec, err := Table.Spec()
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	signed := map[string]bool{"by": true, "author_id": true, "caller_id": true, "stopped_by": true}
	for name, op := range spec.Operations {
		for _, arg := range op.Args {
			if signed[arg.As] && arg.From != dispatch.FromCaller {
				t.Errorf("%s reads %s from %s; the caller signs it", name, arg.As, arg.From)
			}
			if arg.As == "actor_id" && arg.From == dispatch.FromBody && !namesAnotherPersonByDesign[name] {
				t.Errorf("%s reads actor_id from the body but does not name another person by design", name)
			}
		}
	}
}
