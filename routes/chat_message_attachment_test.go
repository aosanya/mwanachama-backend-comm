package routes_test

import (
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func (f roomFixture) postWith(body map[string]string) reply {
	f.t.Helper()
	return f.call(http.MethodPost, "/groups/s-1/chat/threads/"+f.threadID+"/messages", "actor-b", body)
}

func TestAPostCarriesOneAttachmentAndReadsBackWithIt(t *testing.T) {
	f := newRoomFixture(t)
	got := f.postWith(map[string]string{"body": "please fill this", "attachment_kind": "form", "attachment_id": "form-123"})
	if got.code != http.StatusCreated {
		t.Fatalf("post = %d: %s", got.code, got.body)
	}
	var posted mwanachamacomm.ChatMessage
	got.decode(t, &posted)
	if posted.AttachmentKind != "form" || posted.AttachmentID != "form-123" {
		t.Fatalf("posted = %+v, want the attachment returned", posted)
	}

	read := f.call(http.MethodGet, "/chat/messages/"+posted.ID, "actor-a", nil)
	if read.code != http.StatusOK {
		t.Fatalf("read = %d: %s", read.code, read.body)
	}
	var one mwanachamacomm.ChatMessage
	read.decode(t, &one)
	if one.AttachmentKind != "form" || one.AttachmentID != "form-123" {
		t.Fatalf("read = %+v, want the attachment", one)
	}

	list := f.messages(f.threadID, "actor-a", "")
	var msgs []mwanachamacomm.ChatMessage
	list.decode(t, &msgs)
	if len(msgs) != 1 || msgs[0].AttachmentID != "form-123" {
		t.Fatalf("list = %+v, want the attachment", msgs)
	}
}

func TestAPostWithoutAnAttachmentStillWorks(t *testing.T) {
	f := newRoomFixture(t)
	msg := f.post(f.threadID, "actor-b", "plain")
	if msg.AttachmentKind != "" || msg.AttachmentID != "" {
		t.Fatalf("plain post = %+v, want no attachment", msg)
	}
}

func TestAPostNamingOnlyHalfAnAttachmentIsRefused(t *testing.T) {
	f := newRoomFixture(t)
	for name, body := range map[string]map[string]string{
		"kind only": {"body": "x", "attachment_kind": "form"},
		"id only":   {"body": "x", "attachment_id": "form-123"},
		"bad id":    {"body": "x", "attachment_kind": "form", "attachment_id": "no spaces/allowed"},
		"bad kind":  {"body": "x", "attachment_kind": "Form", "attachment_id": "form-123"},
	} {
		if got := f.postWith(body); got.code != http.StatusBadRequest {
			t.Fatalf("%s = %d: %s, want 400", name, got.code, got.body)
		}
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "")); len(got) != 0 {
		t.Fatalf("refused posts stored %v", got)
	}
}
