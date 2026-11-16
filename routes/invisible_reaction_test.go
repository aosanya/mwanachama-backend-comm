package routes_test

import (
	"net/http"
	"strings"
	"testing"
)

var reactionsThatDrawNothing = map[string]string{
	"zero width space":    "\u200b",
	"hangul filler":       "\u3164",
	"plain space":         " ",
	"no break space":      "\u00a0",
	"word joiner":         "\u2060",
	"variation selector":  "\ufe0f",
	"spaces and a joiner": " \u200d ",
}

var reactionsThatDraw = map[string]string{
	"a thumb":             "\U0001f44d",
	"an emoji family":     "\U0001f468\u200d\U0001f469\u200d\U0001f467",
	"a heart presented":   "\u2764\ufe0f",
	"a letter":            "x",
	"a Unicode 16 emoji":  "\U0001fae9",
	"a Unicode 17 shovel": "\U0001fa8f",
}

func TestAReactionThatDrawsNothingIsRefusedLikeAnEmptyOne(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedThread(t, m, "admin-1")
	messageID := seedMessage(t, m, threadID, "admin-1")

	for label, emoji := range reactionsThatDrawNothing {
		t.Run(label, func(t *testing.T) {
			got := call(t, srv, http.MethodPut, "/dm/messages/"+messageID+"/reaction", "admin-1",
				map[string]string{"emoji": emoji})
			if got.code != http.StatusBadRequest || !strings.Contains(string(got.body), "emoji is required") {
				t.Fatalf("%q = %d %s, want 400 emoji is required", emoji, got.code, got.body)
			}
		})
	}
}

func TestAReactionThatDrawsSomethingIsKept(t *testing.T) {
	srv, m := newCommServer(t)
	threadID := seedThread(t, m, "admin-1")
	messageID := seedMessage(t, m, threadID, "admin-1")

	for label, emoji := range reactionsThatDraw {
		t.Run(label, func(t *testing.T) {
			got := call(t, srv, http.MethodPut, "/dm/messages/"+messageID+"/reaction", "admin-1",
				map[string]string{"emoji": emoji})
			if got.code != http.StatusNoContent {
				t.Fatalf("%q = %d %s, want 204", emoji, got.code, got.body)
			}
		})
	}
}
