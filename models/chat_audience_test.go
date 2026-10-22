package models

import (
	"testing"
	"time"
)

func TestAThreadWithTheLeadsIsReadByTheLeadsAndWhoAskedForIt(t *testing.T) {
	leads := ChatThread{Audience: ChatAudienceLeads, RequestedBy: "asker"}
	cases := []struct {
		name   string
		thread ChatThread
		actor  string
		lead   bool
		want   bool
	}{
		{"a lead", leads, "someone", true, true},
		{"the actor who asked", leads, "asker", false, true},
		{"anybody else", leads, "someone", false, false},
		{"nobody, on a thread nobody asked for", ChatThread{Audience: ChatAudienceLeads}, "", false, false},
		{"anybody, once it is the group's", ChatThread{Audience: ChatAudienceGroup}, "someone", false, true},
		{"anybody, on a row older than the audience", ChatThread{}, "someone", false, true},
	}
	for _, c := range cases {
		if got := c.thread.ReadableBy(c.actor, c.lead); got != c.want {
			t.Errorf("%s: ReadableBy = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestADeclinedThreadIsReadableByItsAskerAndTheLeadsAndDeclinedOnce(t *testing.T) {
	at := time.Unix(0, 0)
	declined := ChatThread{Audience: ChatAudienceLeads, RequestedBy: "asker", DeclinedAt: &at}
	if !declined.ReadableBy("asker", false) || !declined.ReadableBy("someone", true) {
		t.Fatal("a declined thread is hidden from its asker or the leads")
	}
	if declined.ReadableBy("someone", false) {
		t.Fatal("a declined thread is readable by a member who neither asked nor leads")
	}
	cases := []struct {
		name   string
		thread ChatThread
		want   bool
	}{
		{"a request still with the leads", ChatThread{Audience: ChatAudienceLeads, RequestedBy: "asker"}, true},
		{"a thread the leads started", ChatThread{Audience: ChatAudienceLeads}, false},
		{"a request already opened", ChatThread{Audience: ChatAudienceGroup, RequestedBy: "asker"}, false},
		{"a request already declined", declined, false},
	}
	for _, c := range cases {
		if got := c.thread.Declinable(); got != c.want {
			t.Errorf("%s: Declinable = %v, want %v", c.name, got, c.want)
		}
	}
}
