package routes_test

import (
	"net/http"
	"reflect"
	"testing"
)

func seedFiveInTheThread(f roomFixture) []string {
	ids := []string{}
	for _, body := range []string{"one", "two", "three", "four", "five"} {
		ids = append(ids, f.post(f.threadID, "actor-b", body).ID)
	}
	return ids
}

func TestAListingWithALimitReturnsTheNewestPostsOldestFirst(t *testing.T) {
	f := newRoomFixture(t)
	seedFiveInTheThread(f)
	f.post(f.other, "actor-b", "elsewhere")

	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?limit=2")); !reflect.DeepEqual(got, []string{"four", "five"}) {
		t.Fatalf("limit 2 = %v, want four and five", got)
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?limit=50")); len(got) != 5 || got[0] != "one" {
		t.Fatalf("limit 50 = %v, want all five oldest first", got)
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "")); len(got) != 5 {
		t.Fatalf("without limit = %v, want all five", got)
	}
}

func TestAListingWithABeforeReturnsThePostsBeforeIt(t *testing.T) {
	f := newRoomFixture(t)
	ids := seedFiveInTheThread(f)

	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?before="+ids[4]+"&limit=2")); !reflect.DeepEqual(got, []string{"three", "four"}) {
		t.Fatalf("2 before five = %v, want three and four", got)
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?before="+ids[2])); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("before three = %v, want one and two", got)
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?before="+ids[0]+"&limit=3")); len(got) != 0 {
		t.Fatalf("before the first = %v, want nothing", got)
	}
}

func TestAnAfterAndALimitKeepTheNewestOfWhatFollows(t *testing.T) {
	f := newRoomFixture(t)
	ids := seedFiveInTheThread(f)

	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?after="+ids[0]+"&limit=2")); !reflect.DeepEqual(got, []string{"four", "five"}) {
		t.Fatalf("2 after one = %v, want four and five", got)
	}
	if got := bodiesOf(t, f.messages(f.threadID, "actor-a", "?after="+ids[0]+"&before="+ids[4])); !reflect.DeepEqual(got, []string{"two", "three", "four"}) {
		t.Fatalf("between one and five = %v, want two, three and four", got)
	}
}

func TestABeforeOutsideTheThreadAnswersNotFound(t *testing.T) {
	f := newRoomFixture(t)
	seedFiveInTheThread(f)
	elsewhere := f.post(f.other, "actor-b", "elsewhere")

	for name, query := range map[string]string{
		"another thread's post": "?before=" + elsewhere.ID + "&limit=2",
		"an invented post":      "?before=msg-invented",
	} {
		if got := f.messages(f.threadID, "actor-a", query); got.code != http.StatusNotFound {
			t.Fatalf("%s = %d: %s, want 404", name, got.code, got.body)
		}
	}
}

func TestAnOutsiderNamingABeforeOrALimitStillAnswersNotFound(t *testing.T) {
	f := newRoomFixture(t)
	ids := seedFiveInTheThread(f)

	for name, query := range map[string]string{
		"a before": "?before=" + ids[4],
		"a limit":  "?limit=2",
	} {
		if got := f.messages(f.threadID, "actor-outsider", query); got.code != http.StatusNotFound {
			t.Fatalf("outsider with %s = %d: %s, want 404", name, got.code, got.body)
		}
	}
}

func TestANegativeLimitIsRefused(t *testing.T) {
	f := newRoomFixture(t)
	seedFiveInTheThread(f)

	if got := f.messages(f.threadID, "actor-a", "?limit=-1"); got.code != http.StatusBadRequest {
		t.Fatalf("limit -1 = %d: %s, want 400", got.code, got.body)
	}
}
