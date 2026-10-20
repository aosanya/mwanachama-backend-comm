package models

import "testing"

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
