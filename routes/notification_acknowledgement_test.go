package routes_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

func raiseTo(t *testing.T, m interface {
	Raise(context.Context, models.Notification) (models.Notification, error)
}, actor, subjectID string) models.Notification {
	t.Helper()
	n, err := m.Raise(context.Background(), models.Notification{
		ActorID: actor, Category: models.CategoryItem, Event: models.EventItemFiledForYou,
		SubjectKind: models.SubjectItem, SubjectID: subjectID,
	})
	if err != nil {
		t.Fatalf("Raise to %s: %v", actor, err)
	}
	return n
}

func TestARecipientAcknowledgesAndTheCentreSeesWhoDid(t *testing.T) {
	srv, m := newCommServer(t)
	one := raiseTo(t, m, "ward-1", "instruction-1")
	raiseTo(t, m, "ward-2", "instruction-1")

	if got := call(t, srv, http.MethodPost, "/notifications/"+one.ID+"/acknowledge", "ward-1", nil); got.code != http.StatusOK {
		t.Fatalf("acknowledge = %d: %s", got.code, got.body)
	}

	seen := call(t, srv, http.MethodGet, "/notification-receipts?subject_kind=item&subject_id=instruction-1", "centre-1", nil)
	if seen.code != http.StatusOK {
		t.Fatalf("receipts = %d: %s", seen.code, seen.body)
	}
	var out []models.NotificationReceipt
	seen.decode(t, &out)
	if len(out) != 2 {
		t.Fatalf("receipts = %s, want two recipients", seen.body)
	}
	for _, r := range out {
		acknowledged := r.AcknowledgedAt != nil
		if acknowledged != (r.ActorID == "ward-1") {
			t.Fatalf("receipt %+v: only ward-1 acknowledged", r)
		}
	}
}

func TestSomebodyElsesNotificationIsNotYoursToAcknowledge(t *testing.T) {
	srv, m := newCommServer(t)
	one := raiseTo(t, m, "ward-1", "instruction-1")

	if got := call(t, srv, http.MethodPost, "/notifications/"+one.ID+"/acknowledge", "ward-2", nil); got.code != http.StatusNotFound {
		t.Fatalf("acknowledging another's = %d, want 404: %s", got.code, got.body)
	}
	if got := call(t, srv, http.MethodPost, "/notifications/nothing/acknowledge", "ward-1", nil); got.code != http.StatusNotFound {
		t.Fatalf("acknowledging an unknown one = %d, want 404: %s", got.code, got.body)
	}
}

func TestReceiptsWithoutASubjectAreRefused(t *testing.T) {
	srv, _ := newCommServer(t)
	if got := call(t, srv, http.MethodGet, "/notification-receipts?subject_kind=item", "centre-1", nil); got.code != http.StatusBadRequest {
		t.Fatalf("receipts without a subject id = %d, want 400: %s", got.code, got.body)
	}
}

func TestReceiptsAreGatedByTheirOwnAction(t *testing.T) {
	_, m := newCommTestDB(t)
	only := func(allowed string) dispatch.Authorizer {
		return func(ctx context.Context, action string) error {
			if action != allowed {
				return dispatch.ErrForbidden
			}
			return nil
		}
	}
	one := raiseTo(t, m, "ward-1", "instruction-1")
	srv := newServer(t, m, routes.Mount{
		Caller: fromCtx(callerKey), Device: fromCtx(deviceKey),
		Authorize: only("comm.notification.acknowledge"),
	})

	if got := call(t, srv, http.MethodPost, "/notifications/"+one.ID+"/acknowledge", "ward-1", nil); got.code != http.StatusOK {
		t.Fatalf("acknowledge with only its own action = %d: %s", got.code, got.body)
	}
	if got := call(t, srv, http.MethodGet, "/notification-receipts?subject_kind=item&subject_id=instruction-1", "ward-1", nil); got.code != http.StatusForbidden {
		t.Fatalf("receipts with only the acknowledge action = %d, want 403", got.code)
	}
}
