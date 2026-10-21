package routes

import (
	"fmt"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

type Tool = dispatch.Tool

var ChatActions = []string{
	"comm.chat_thread.list",
	"comm.chat_message.list",
	"comm.chat_message.post",
	"comm.chat_thread.mark_read",
	"comm.chat_thread.create",
}

func BuildChatTools(m *mwanachamacomm.CommManager, mount Mount) ([]Tool, error) {
	s, err := dispatch.Parse(mwanachamacomm.Operations())
	if err != nil {
		return nil, err
	}
	all, err := dispatch.Tools(s, dispatch.Deps{
		Manager:   m,
		Errors:    Sentinels,
		Authorize: mount.Authorize,
		Caller:    mount.Caller,
		Device:    mount.Device,
	})
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, a := range ChatActions {
		wanted[a] = true
	}
	var out []Tool
	for _, t := range all {
		if wanted[t.Action] {
			out = append(out, t)
			delete(wanted, t.Action)
		}
	}
	if len(wanted) > 0 {
		return nil, fmt.Errorf("comm tools: no tool for %v", wanted)
	}
	return out, nil
}

func ChatTools(m *mwanachamacomm.CommManager, mount Mount) []Tool {
	out, err := BuildChatTools(m, mount)
	if err != nil {
		panic(fmt.Sprintf("comm tools: %v", err))
	}
	return out
}
