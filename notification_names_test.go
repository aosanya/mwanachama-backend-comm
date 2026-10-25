package mwanachamacomm_test

import (
	"context"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func TestANoticeCarriesTheNamesItWasRaisedWith(t *testing.T) {
	s := newNotificationStore(t)
	ctx := context.Background()
	raise := func(subject string, names map[string]string) {
		t.Helper()
		if _, err := s.Raise(ctx, mwanachamacomm.Notification{
			ActorID:     "m-1",
			Category:    mwanachamacomm.CategoryResults,
			Event:       mwanachamacomm.EventResultsPublished,
			SubjectKind: mwanachamacomm.SubjectSurvey,
			SubjectID:   subject,
			Names:       names,
		}); err != nil {
			t.Fatalf("Raise: %v", err)
		}
	}
	raise("survey-1", map[string]string{"group": "Ward 7", "author": "Amina", "title": "Water survey"})
	raise("survey-1", map[string]string{"group": "Ward Seven", "author": "Amina K", "title": "Water survey, revised"})
	raise("survey-2", nil)

	out, err := s.List(ctx, "m-1", 0)
	if err != nil || len(out) != 3 {
		t.Fatalf("List = %+v, err %v, want 3 rows", out, err)
	}
	bySubjectTitle := map[string]map[string]string{}
	for _, n := range out {
		key := n.SubjectID + "|" + n.Names["title"]
		bySubjectTitle[key] = n.Names
	}
	first := bySubjectTitle["survey-1|Water survey"]
	if first["group"] != "Ward 7" || first["author"] != "Amina" {
		t.Fatalf("the first notice's names = %v, want the names it was raised with", first)
	}
	second := bySubjectTitle["survey-1|Water survey, revised"]
	if second["group"] != "Ward Seven" || second["author"] != "Amina K" {
		t.Fatalf("the second notice's names = %v", second)
	}
	if bare, ok := bySubjectTitle["survey-2|"]; !ok || len(bare) != 0 {
		t.Fatalf("a notice raised without names reads back %v, want none", bare)
	}
}
