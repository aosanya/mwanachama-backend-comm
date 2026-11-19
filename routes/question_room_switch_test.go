package routes

import (
	"encoding/json"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func TestTheQuestionRoomSwitchTakesItsArgumentsInTheOrderOfItsMethod(t *testing.T) {
	var spec struct {
		Operations map[string]struct {
			Args []struct {
				As string `json:"as"`
			} `json:"args"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(mwanachamacomm.Operations(), &spec); err != nil {
		t.Fatalf("operations: %v", err)
	}
	op, ok := spec.Operations["set_chat_question_room"]
	if !ok {
		t.Fatal("no set_chat_question_room operation")
	}
	want := []string{"structure_id", "caller_id", "on"}
	if len(op.Args) != len(want) {
		t.Fatalf("args = %v, want %v", op.Args, want)
	}
	for i, arg := range op.Args {
		if arg.As != want[i] {
			t.Fatalf("argument %d is %q, want %q: the method takes (ctx, structureID, callerID, on) in that order", i, arg.As, want[i])
		}
	}
}

func TestARefusedPostIntoAQuestionRoomAnswersAsAConflictNotAnInternalError(t *testing.T) {
	var spec struct {
		Errors map[string]int `json:"errors"`
	}
	if err := json.Unmarshal(mwanachamacomm.Operations(), &spec); err != nil {
		t.Fatalf("operations: %v", err)
	}
	if spec.Errors["ErrChatQuestionRoom"] != 409 {
		t.Fatalf("ErrChatQuestionRoom answers %d, want 409", spec.Errors["ErrChatQuestionRoom"])
	}
	if Sentinels["ErrChatQuestionRoom"] != mwanachamacomm.ErrChatQuestionRoom {
		t.Fatal("the sentinel table does not name ErrChatQuestionRoom")
	}
}
