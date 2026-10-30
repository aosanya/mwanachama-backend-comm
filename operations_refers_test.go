package mwanachamacomm

import (
	"sort"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

var markedReferences = map[string]string{
	"open_chat_thread.thread_id":               "comm.chat_thread",
	"decline_chat_thread.thread_id":            "comm.chat_thread",
	"list_chat_messages.thread_id":             "comm.chat_thread",
	"post_chat_message.thread_id":              "comm.chat_thread",
	"mark_chat_thread_read.thread_id":          "comm.chat_thread",
	"archive_chat_thread.thread_id":            "comm.chat_thread",
	"unarchive_chat_thread.thread_id":          "comm.chat_thread",
	"create_standing_message.thread_id":        "comm.chat_thread",
	"get_chat_message.message_id":              "comm.chat_message",
	"mark_chat_thread_read.message_id":         "comm.chat_message",
	"list_reports_for_message.message_id":      "comm.chat_message",
	"list_chat_messages.after":                 "comm.chat_message",
	"get_subject_thread.thread_id":             "comm.subject_thread",
	"list_subject_messages.thread_id":          "comm.subject_thread",
	"post_subject_message.thread_id":           "comm.subject_thread",
	"acknowledge_notification.notification_id": "comm.notification",
	"list_push_deliveries.notification_id":     "comm.notification",
	"list_notification_receipts.after":         "comm.notification",
	"mark_notifications_read.ids":              "comm.notification",
	"get_standing_message.standing_id":         "comm.standing_message",
	"stop_standing_message.standing_id":        "comm.standing_message",
}

func referenceKey(r dispatch.Reference) string { return r.Operation + "." + r.As }

func TestEveryArgumentNamingAThingCommKeepsIsMarkedWithWhatItRefersTo(t *testing.T) {
	s, err := dispatch.Parse(Operations())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	listed := map[string]string{}
	for _, r := range s.References() {
		listed[referenceKey(r)] = r.Refers
	}
	var wrong []string
	for key, want := range markedReferences {
		if got := listed[key]; got != want {
			wrong = append(wrong, key+" refers "+got+", want "+want)
		}
	}
	for key, got := range listed {
		if _, ok := markedReferences[key]; !ok {
			wrong = append(wrong, key+" refers "+got+" and is not one of the things comm keeps")
		}
	}
	sort.Strings(wrong)
	if len(wrong) > 0 {
		t.Fatalf("references drifted:\n%s", strings.Join(wrong, "\n"))
	}
}

func TestEveryReferenceNamesAnObjectTheStableIdentityAnswersFor(t *testing.T) {
	s, err := dispatch.Parse(Operations())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, r := range s.References() {
		module, object, _ := strings.Cut(r.Refers, ".")
		if module != identityModule || !identifiedRoles[object] {
			t.Errorf("%s refers %q, which comm's owner cannot resolve", referenceKey(r), r.Refers)
		}
	}
}

func TestNoSessionFactOrDirectMessageIsMarkedAsAReference(t *testing.T) {
	s, err := dispatch.Parse(Operations())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, r := range s.References() {
		if r.From == dispatch.FromCaller || r.From == dispatch.FromDevice {
			t.Errorf("%s comes from the session", referenceKey(r))
		}
		if strings.HasPrefix(r.Operation, "dm_") || strings.Contains(r.Operation, "_dm_") || strings.HasSuffix(r.Operation, "_dm_thread") {
			t.Errorf("%s names a direct message, which has no stable identity", referenceKey(r))
		}
	}
}
