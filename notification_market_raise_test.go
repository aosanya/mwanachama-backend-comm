package mwanachamacomm_test

import (
	"context"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestAMarketNoticeIsKeptForOnePersonAndNobodyElsesList(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	n := mwanachamacomm.Notification{ActorID: "supplier-1", Category: models.CategoryMarket, Event: models.EventSupplierApproved, SubjectKind: models.SubjectPoster, SubjectID: "poster-1"}
	if _, err := s.Raise(ctx, n); err != nil {
		t.Fatalf("raise: %v", err)
	}
	mine, err := s.List(ctx, "supplier-1", 0)
	if err != nil || len(mine) != 1 || mine[0].Event != models.EventSupplierApproved {
		t.Fatalf("addressee list: %+v %v", mine, err)
	}
	other, err := s.List(ctx, "farmer-1", 0)
	if err != nil || len(other) != 0 {
		t.Fatalf("non-addressee list: %+v %v", other, err)
	}
}
