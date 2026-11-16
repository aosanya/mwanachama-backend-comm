package routes_test

import (
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

var messagesThatDrawNothing = map[string]string{
	"zero width spaces":      "\u200b\u200b\u200b",
	"word joiner":            "\u2060",
	"left to right mark":     "\u200e",
	"byte order mark":        "\ufeff",
	"soft hyphen":            "\u00ad",
	"hangul filler":          "\u3164\u3164",
	"halfwidth hangul":       "\uffa0",
	"braille blank":          "\u2800",
	"bidi marks and spaces":  " \u202e \u200f\n\t\u2066 ",
	"lone combining mark":    "\u0301",
	"private use":            "\ue000",
	"variation selector":     "\ufe0f",
	"invisible between rows": "\u200b\n\u200b\n\u200b",
}

var messagesThatDraw = map[string]string{
	"a real message":                        "Hello, the bin lorry is late.",
	"an accented Hungarian message":         "Árvíztűrő tükörfúrógép, köszönöm szépen!",
	"a decomposed accent":                   "Kovacs\u0301 Adam",
	"a message over several lines":          "First line\n\n\tSecond line",
	"an emoji joined by a zero width":       "\U0001f468\u200d\U0001f469\u200d\U0001f467",
	"a zero width space between real words": "bin\u200blorry",
	"punctuation alone":                     "?!",
	"a digit alone":                         "7",
}

func TestAChatPostThatDrawsNothingIsRefused(t *testing.T) {
	f := newRoomFixture(t)
	for label, text := range messagesThatDrawNothing {
		t.Run(label, func(t *testing.T) {
			got := f.call(http.MethodPost, "/groups/s-1/chat/threads/"+f.threadID+"/messages", "actor-a", map[string]string{"body": text})
			if got.code != http.StatusBadRequest {
				t.Fatalf("%q = %d, want 400: %s", text, got.code, got.body)
			}
		})
	}
	if stored := bodiesOf(t, f.messages(f.threadID, "actor-a", "")); len(stored) != 0 {
		t.Fatalf("refused posts stored %q", stored)
	}
}

func TestAChatPostThatDrawsSomethingIsKept(t *testing.T) {
	f := newRoomFixture(t)
	for label, text := range messagesThatDraw {
		t.Run(label, func(t *testing.T) {
			got := f.call(http.MethodPost, "/groups/s-1/chat/threads/"+f.threadID+"/messages", "actor-a", map[string]string{"body": text})
			if got.code != http.StatusCreated {
				t.Fatalf("%q = %d, want 201: %s", text, got.code, got.body)
			}
		})
	}
	if stored := bodiesOf(t, f.messages(f.threadID, "actor-a", "")); len(stored) != len(messagesThatDraw) {
		t.Fatalf("stored %d of %d accepted posts", len(stored), len(messagesThatDraw))
	}
}

func TestASubjectPostThatDrawsNothingIsRefused(t *testing.T) {
	c, _ := subjectServer(t)
	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")
	for label, text := range messagesThatDrawNothing {
		t.Run(label, func(t *testing.T) {
			got := c(http.MethodPost, "/subject-threads/"+group.ID+"/messages", "actor-a", map[string]string{"body": text})
			if got.code != http.StatusBadRequest {
				t.Fatalf("%q = %d, want 400: %s", text, got.code, got.body)
			}
		})
	}
	if refetched := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group"); refetched.MessageCount != 0 {
		t.Fatalf("refused posts left %d messages", refetched.MessageCount)
	}
}

func TestASubjectPostThatDrawsSomethingIsKept(t *testing.T) {
	c, _ := subjectServer(t)
	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")
	for label, text := range messagesThatDraw {
		t.Run(label, func(t *testing.T) {
			got := c(http.MethodPost, "/subject-threads/"+group.ID+"/messages", "actor-a", map[string]string{"body": text})
			if got.code != http.StatusCreated {
				t.Fatalf("%q = %d, want 201: %s", text, got.code, got.body)
			}
		})
	}
}

func TestAStandingMessageThatDrawsNothingIsRefused(t *testing.T) {
	do, _, thread := newStandingFixture(t)
	for label, text := range messagesThatDrawNothing {
		t.Run(label, func(t *testing.T) {
			body := standingBody(thread)
			body["body"] = text
			got := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", body)
			if got.code != http.StatusBadRequest {
				t.Fatalf("%q = %d, want 400: %s", text, got.code, got.body)
			}
		})
	}
	listed := do(http.MethodGet, "/groups/s-1/chat/standing", "actor-a", nil)
	var kept []mwanachamacomm.StandingMessage
	listed.decode(t, &kept)
	if listed.code != http.StatusOK || len(kept) != 0 {
		t.Fatalf("refused standing messages left %s", listed.body)
	}
}

func TestAStandingMessageThatDrawsSomethingIsKept(t *testing.T) {
	do, _, thread := newStandingFixture(t)
	for label, text := range messagesThatDraw {
		t.Run(label, func(t *testing.T) {
			body := standingBody(thread)
			body["body"] = text
			got := do(http.MethodPost, "/groups/s-1/chat/standing", "actor-a", body)
			if got.code != http.StatusCreated {
				t.Fatalf("%q = %d, want 201: %s", text, got.code, got.body)
			}
		})
	}
}
