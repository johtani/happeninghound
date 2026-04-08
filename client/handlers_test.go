package client

import (
	"testing"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func TestSlashCommandFromEventData(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
		want bool
	}{
		{
			name: "slash command",
			data: slack.SlashCommand{
				Command:   "/make-html",
				ChannelID: "C123",
			},
			want: true,
		},
		{
			name: "nil",
			data: nil,
			want: false,
		},
		{
			name: "string",
			data: "unexpected",
			want: false,
		},
		{
			name: "events api event",
			data: slackevents.EventsAPIEvent{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := slashCommandFromEventData(tt.data)
			if ok != tt.want {
				t.Fatalf("slashCommandFromEventData() ok = %v, want %v", ok, tt.want)
			}
		})
	}
}
