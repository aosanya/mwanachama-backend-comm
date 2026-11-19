package mwanachamacomm

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func questionRoomFixture(t *testing.T) (*cover13Fixture, models.ChatThread) {
	t.Helper()
	f := cover13Setup(t)
	wireReach(f, map[string]map[string]bool{"s-1": {"mem-1": true, "mem-2": true, "lead-1": true}}, map[string]bool{"lead-1": true})
	group, err := f.m.MintThread(context.Background(), "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	return f, group
}

func TestQuestionRoomsAreNoneUntilTheOwnerNamesOne(t *testing.T) {
	f, _ := questionRoomFixture(t)
	rooms, err := f.m.QuestionRooms(context.Background())
	if err != nil {
		t.Fatalf("QuestionRooms: %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("question rooms = %v, want none by default", rooms)
	}
}

func TestAnOrdinaryRoomStillLetsAMemberPostInTheGroupThread(t *testing.T) {
	f, group := questionRoomFixture(t)
	if _, err := f.m.PostChatMessageFor(context.Background(), "s-1", group.ID, "mem-1", "Hello all", "", ""); err != nil {
		t.Fatalf("a member's post in an ordinary room = %v, want it accepted", err)
	}
}

func TestAQuestionRoomRefusesAMemberPostingIntoTheOpenGroupThread(t *testing.T) {
	f, group := questionRoomFixture(t)
	ctx := context.Background()
	if err := f.m.SetQuestionRooms(ctx, []string{"s-1"}, "owner-1"); err != nil {
		t.Fatalf("SetQuestionRooms: %v", err)
	}
	_, err := f.m.PostChatMessageFor(ctx, "s-1", group.ID, "mem-1", "When does the permit office open?", "", "")
	if !errors.Is(err, models.ErrChatQuestionRoom) {
		t.Fatalf("a member's post in the open thread of a question room = %v, want ErrChatQuestionRoom", err)
	}
	read, err := f.m.ListChatMessagesFor(ctx, "s-1", group.ID, "mem-2", "", "", 50)
	if err != nil {
		t.Fatalf("ListChatMessagesFor: %v", err)
	}
	if len(read) != 0 {
		t.Fatalf("the refused question reached the room: %+v", read)
	}
}

func TestAQuestionRoomLetsTheLeadersPostInTheGroupThread(t *testing.T) {
	f, group := questionRoomFixture(t)
	ctx := context.Background()
	if err := f.m.SetQuestionRooms(ctx, []string{"s-1"}, "owner-1"); err != nil {
		t.Fatalf("SetQuestionRooms: %v", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", group.ID, "lead-1", "The office opens at nine", "", ""); err != nil {
		t.Fatalf("a leader's post in a question room = %v, want it accepted", err)
	}
}

func TestInAQuestionRoomAMemberAsksInTheirOwnThreadAndNeighboursReadNothingOfIt(t *testing.T) {
	f, _ := questionRoomFixture(t)
	ctx := context.Background()
	if err := f.m.SetQuestionRooms(ctx, []string{"s-1"}, "owner-1"); err != nil {
		t.Fatalf("SetQuestionRooms: %v", err)
	}
	asked, err := f.m.StartChatThreadFor(ctx, "s-1", "mem-1", "Permit question")
	if err != nil {
		t.Fatalf("StartChatThreadFor: %v", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", asked.ID, "mem-1", "When does the permit office open?", "", ""); err != nil {
		t.Fatalf("the asker's post in their own thread = %v", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", asked.ID, "lead-1", "At nine on weekdays", "", ""); err != nil {
		t.Fatalf("the leader's answer = %v", err)
	}
	if read, err := f.m.ListChatMessagesFor(ctx, "s-1", asked.ID, "mem-2", "", "", 50); err == nil && len(read) > 0 {
		t.Fatalf("a neighbour read the question and its answer: %+v", read)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", asked.ID, "mem-2", "Me too", "", ""); err == nil {
		t.Fatal("a neighbour posted into another member's question")
	}
	own, err := f.m.ListChatMessagesFor(ctx, "s-1", asked.ID, "mem-1", "", "", 50)
	if err != nil || len(own) != 2 {
		t.Fatalf("the asker reads %d messages, err %v, want their question and its answer", len(own), err)
	}
	led, err := f.m.ListChatMessagesFor(ctx, "s-1", asked.ID, "lead-1", "", "", 50)
	if err != nil || len(led) != 2 {
		t.Fatalf("the leader reads %d messages, err %v, want both", len(led), err)
	}
}

func TestASecondRoomIsNotMadeAQuestionRoomByTheFirst(t *testing.T) {
	f, group := questionRoomFixture(t)
	ctx := context.Background()
	if err := f.m.SetQuestionRooms(ctx, []string{"s-2"}, "owner-1"); err != nil {
		t.Fatalf("SetQuestionRooms: %v", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", group.ID, "mem-1", "Hello all", "", ""); err != nil {
		t.Fatalf("a member's post in a room that is not a question room = %v", err)
	}
}

func TestThreadViewsSayWhichRoomsAreQuestionRooms(t *testing.T) {
	f, _ := questionRoomFixture(t)
	ctx := context.Background()
	before, err := f.m.ListChatThreadsFor(ctx, "s-1", "mem-1")
	if err != nil {
		t.Fatalf("ListChatThreadsFor: %v", err)
	}
	for _, view := range before {
		if view.QuestionRoom {
			t.Fatalf("thread %s says question room before the owner named one", view.ID)
		}
	}
	if err := f.m.SetQuestionRooms(ctx, []string{"s-1"}, "owner-1"); err != nil {
		t.Fatalf("SetQuestionRooms: %v", err)
	}
	after, err := f.m.ListChatThreadsFor(ctx, "s-1", "mem-1")
	if err != nil {
		t.Fatalf("ListChatThreadsFor: %v", err)
	}
	if len(after) == 0 {
		t.Fatal("the member sees no thread")
	}
	for _, view := range after {
		if !view.QuestionRoom {
			t.Fatalf("thread %s does not say question room after the owner named the room", view.ID)
		}
	}
}

func TestALeaderSwitchesARoomIntoAQuestionRoomAndBack(t *testing.T) {
	f, group := questionRoomFixture(t)
	ctx := context.Background()
	set, err := f.m.SetChatQuestionRoomFor(ctx, "s-1", "lead-1", true)
	if err != nil {
		t.Fatalf("SetChatQuestionRoomFor: %v", err)
	}
	if !set.QuestionRoom || set.StructureID != "s-1" {
		t.Fatalf("switch result = %+v, want the room on", set)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", group.ID, "mem-1", "hello", "", ""); !errors.Is(err, models.ErrChatQuestionRoom) {
		t.Fatalf("a member's post after the switch = %v, want ErrChatQuestionRoom", err)
	}
	off, err := f.m.SetChatQuestionRoomFor(ctx, "s-1", "lead-1", false)
	if err != nil {
		t.Fatalf("SetChatQuestionRoomFor off: %v", err)
	}
	if off.QuestionRoom {
		t.Fatalf("switch result = %+v, want the room off", off)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", group.ID, "mem-1", "hello", "", ""); err != nil {
		t.Fatalf("a member's post after switching back = %v, want it accepted", err)
	}
}

func TestOnlyALeaderMaySwitchARoomIntoAQuestionRoom(t *testing.T) {
	f, _ := questionRoomFixture(t)
	if _, err := f.m.SetChatQuestionRoomFor(context.Background(), "s-1", "mem-1", true); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("a member's switch = %v, want the same not-found as any outsider", err)
	}
	rooms, err := f.m.QuestionRooms(context.Background())
	if err != nil || len(rooms) != 0 {
		t.Fatalf("question rooms = %v, %v, want none after a refused switch", rooms, err)
	}
}

func TestSwitchingARoomTwiceKeepsOneEntry(t *testing.T) {
	f, _ := questionRoomFixture(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := f.m.SetChatQuestionRoomFor(ctx, "s-1", "lead-1", true); err != nil {
			t.Fatalf("SetChatQuestionRoomFor: %v", err)
		}
	}
	rooms, err := f.m.QuestionRooms(ctx)
	if err != nil || len(rooms) != 1 {
		t.Fatalf("question rooms = %v, %v, want exactly one entry", rooms, err)
	}
}

func TestAQuestionRoomIsNotMarkedForItsLeadersWhoStillPostInTheOpenThread(t *testing.T) {
	f, _ := questionRoomFixture(t)
	ctx := context.Background()
	if err := f.m.SetQuestionRooms(ctx, []string{"s-1"}, "owner-1"); err != nil {
		t.Fatalf("SetQuestionRooms: %v", err)
	}
	views, err := f.m.ListChatThreadsFor(ctx, "s-1", "lead-1")
	if err != nil {
		t.Fatalf("ListChatThreadsFor: %v", err)
	}
	if len(views) == 0 {
		t.Fatal("the leader sees no thread")
	}
	for _, view := range views {
		if view.QuestionRoom {
			t.Fatalf("thread %s is marked question room for a leader, who posts in it", view.ID)
		}
	}
}

func TestThreadViewsTellALeaderThatTheyLeadAndWhetherTheRoomTakesQuestions(t *testing.T) {
	f, _ := questionRoomFixture(t)
	ctx := context.Background()
	lead := func() models.ChatThreadView {
		views, err := f.m.ListChatThreadsFor(ctx, "s-1", "lead-1")
		if err != nil || len(views) == 0 {
			t.Fatalf("ListChatThreadsFor: %v, %d views", err, len(views))
		}
		return views[0]
	}
	if first := lead(); !first.Leader || first.TakesQuestions {
		t.Fatalf("leader view before the switch = leader %v takes questions %v, want a leader and no questions", first.Leader, first.TakesQuestions)
	}
	if err := f.m.SetQuestionRooms(ctx, []string{"s-1"}, "owner-1"); err != nil {
		t.Fatalf("SetQuestionRooms: %v", err)
	}
	if after := lead(); !after.Leader || !after.TakesQuestions {
		t.Fatalf("leader view after the switch = leader %v takes questions %v, want both true", after.Leader, after.TakesQuestions)
	}
	members, err := f.m.ListChatThreadsFor(ctx, "s-1", "mem-1")
	if err != nil || len(members) == 0 {
		t.Fatalf("ListChatThreadsFor: %v", err)
	}
	if members[0].Leader {
		t.Fatal("a member's view says they lead the room")
	}
}
