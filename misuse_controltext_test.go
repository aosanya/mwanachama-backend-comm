package mwanachamacomm

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

var controlTexts = map[string]string{
	"NUL":                "a\u0000b",
	"escape":             "a\u001bb",
	"invalid UTF-8 byte": "a\xffb",
}

func TestDirectMessage_PersonCannotDoStoreControlCharactersInAThreadTitleOrAPayload(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	thread := f.dmThread(t, "mem-1", "mem-1", "mem-2")

	for label, text := range controlTexts {
		t.Run(label, func(t *testing.T) {
			_, err := f.m.DMStore.CreateThread(ctx, models.DMThread{CreatedBy: "mem-1", Title: text}, []string{"mem-1"})
			if !errors.Is(err, ErrInvalidReference) {
				t.Fatalf("a DM thread titled %q: %v, want ErrInvalidReference", text, err)
			}
			_, err = f.m.DMStore.Post(ctx, models.DMMessage{
				ThreadID: thread.ID, SenderID: "mem-1", PayloadCiphertext: text, PerRecipientKeys: map[string]string{},
			})
			if !errors.Is(err, ErrInvalidReference) {
				t.Fatalf("a DM payload %q: %v, want ErrInvalidReference", text, err)
			}
		})
	}
}

func TestNotification_HostCannotDoRaiseOneNamingControlCharacters(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	about := func(subjectID string) models.Notification {
		return models.Notification{
			ActorID:     "mem-1",
			Category:    models.CategorySurvey,
			Event:       models.EventSurveyReached,
			SubjectKind: models.SubjectSurvey,
			SubjectID:   subjectID,
		}
	}
	if _, err := f.m.NotificationStore.Raise(ctx, about("survey-1")); err != nil {
		t.Fatalf("a notification about a plain survey: %v", err)
	}

	for label, text := range controlTexts {
		t.Run(label, func(t *testing.T) {
			_, err := f.m.NotificationStore.Raise(ctx, about(text))
			if !errors.Is(err, models.ErrNotificationInvalid) {
				t.Fatalf("a notification about %q: %v, want ErrNotificationInvalid", text, err)
			}
		})
	}
}

func TestChat_PersonCannotDoStoreControlCharactersInAMessageOrALabel(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	thread, err := f.m.ChatStore.StartThread(ctx, "grp-1", "Plain")
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}

	for label, text := range controlTexts {
		t.Run(label, func(t *testing.T) {
			_, err := f.m.ChatStore.Post(ctx, models.ChatMessage{StructureID: "grp-1", ThreadID: thread.ID, AuthorID: "mem-1", Body: text})
			if !errors.Is(err, ErrInvalidReference) {
				t.Fatalf("a chat message %q: %v, want ErrInvalidReference", text, err)
			}
			if _, err := f.m.ChatStore.StartThread(ctx, "grp-1", text); !errors.Is(err, ErrInvalidReference) {
				t.Fatalf("a thread labelled %q: %v, want ErrInvalidReference", text, err)
			}
			_, _, err = f.m.ChatStore.RequestThread(ctx, "grp-1", text, "mem-2", func(models.ChatThread) bool { return true })
			if !errors.Is(err, ErrInvalidReference) {
				t.Fatalf("a requested thread labelled %q: %v, want ErrInvalidReference", text, err)
			}
		})
	}
}

func TestChat_PersonCanDoWriteMultiLineHungarianText(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	thread, err := f.m.ChatStore.StartThread(ctx, "grp-1", "Szemétszállítás – Árvíztűrő tükörfúrógép \U0001F642")
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	body := "Kedves szomszédok!\n\n\tA zöld kuka csütörtökön megy.\r\nKöszönöm \U0001F44D\U0001F3FD, Éva"
	posted, err := f.m.ChatStore.Post(ctx, models.ChatMessage{StructureID: "grp-1", ThreadID: thread.ID, AuthorID: "mem-1", Body: body})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if posted.Body != body {
		t.Fatalf("body = %q, want %q", posted.Body, body)
	}
}
