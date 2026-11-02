package models

import (
	"testing"
	"time"
)

func TestAThreadSaysWhyAnActorReadsIt(t *testing.T) {
	at := time.Unix(0, 0)
	leads := ChatThread{Audience: ChatAudienceLeads, RequestedBy: "asker"}
	cases := []struct {
		name       string
		thread     ChatThread
		actor      string
		lead       bool
		wantReason ChatReachReason
		wantOK     bool
	}{
		{"a lead", leads, "someone", true, ChatReachLeads, true},
		{"a lead who also asked", leads, "asker", true, ChatReachLeads, true},
		{"the actor who asked", leads, "asker", false, ChatReachAsked, true},
		{"anybody else", leads, "someone", false, "", false},
		{"nobody, on a thread nobody asked for", ChatThread{Audience: ChatAudienceLeads}, "", false, "", false},
		{"a lead, on a thread nobody asked for", ChatThread{Audience: ChatAudienceLeads}, "someone", true, ChatReachLeads, true},
		{"anybody, once it is the group's", ChatThread{Audience: ChatAudienceGroup}, "someone", false, ChatReachGroup, true},
		{"the actor who asked, once it is the group's", ChatThread{Audience: ChatAudienceGroup, RequestedBy: "asker"}, "asker", false, ChatReachGroup, true},
		{"anybody, on a row older than the audience", ChatThread{}, "someone", false, ChatReachGroup, true},
		{"the actor who asked, once declined", ChatThread{Audience: ChatAudienceLeads, RequestedBy: "asker", DeclinedAt: &at}, "asker", false, ChatReachAsked, true},
		{"anybody else, once declined", ChatThread{Audience: ChatAudienceLeads, RequestedBy: "asker", DeclinedAt: &at}, "someone", false, "", false},
	}
	for _, c := range cases {
		reason, ok := c.thread.ReadableReason(c.actor, c.lead)
		if reason != c.wantReason || ok != c.wantOK {
			t.Errorf("%s: ReadableReason = %q, %v; want %q, %v", c.name, reason, ok, c.wantReason, c.wantOK)
		}
		if got := c.thread.ReadableBy(c.actor, c.lead); got != c.wantOK {
			t.Errorf("%s: ReadableBy = %v, want the reason's ok %v", c.name, got, c.wantOK)
		}
	}
}
