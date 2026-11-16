package models

import "testing"

func TestSMSChannelCoversSecurityRoundsAndReports(t *testing.T) {
	for _, c := range []NotificationCategory{CategorySecurity, CategorySurvey, CategoryResults, CategoryContribution} {
		if !NotificationSMSChannel(c) {
			t.Errorf("category %q has no SMS channel", c)
		}
	}
}

func TestSMSChannelStaysOffForChatterCategories(t *testing.T) {
	for _, c := range []NotificationCategory{CategoryChat, CategoryMarket, CategoryPost, CategoryTask, CategoryMerchandise} {
		if NotificationSMSChannel(c) {
			t.Errorf("category %q must not reach a phone by SMS", c)
		}
	}
}

func TestVoiceChannelCoversRoundsAndReports(t *testing.T) {
	for _, c := range []NotificationCategory{CategorySurvey, CategoryResults, CategoryContribution, CategorySecurity} {
		if !NotificationVoiceChannel(c) {
			t.Errorf("category %q has no voice channel", c)
		}
	}
	if NotificationVoiceChannel(CategoryChat) {
		t.Errorf("chat must not ring a phone")
	}
}

func TestChannelsOfListsEveryChannelACategoryHas(t *testing.T) {
	got := NotificationChannels(CategorySurvey)
	want := []NotificationChannel{ChannelSMS, ChannelVoice}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("survey channels = %v, want %v", got, want)
	}
	if len(NotificationChannels(CategoryChat)) != 0 {
		t.Fatalf("chat has screenless channels: %v", NotificationChannels(CategoryChat))
	}
}
