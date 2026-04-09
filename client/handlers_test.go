package client

import (
	"testing"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func TestResolvedChannelName(t *testing.T) {
	tests := []struct {
		name string
		ev   slack.SlashCommand
		want string
	}{
		{
			name: "default channel name",
			ev: slack.SlashCommand{
				ChannelName: "general",
			},
			want: "general",
		},
		{
			name: "text overrides with jsonl suffix",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "dev-team.jsonl",
			},
			want: "dev-team",
		},
		{
			name: "text overrides with spaces",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "  team_1  ",
			},
			want: "team_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvedChannelName(tt.ev); got != tt.want {
				t.Fatalf("resolvedChannelName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateChannelName(t *testing.T) {
	tests := []struct {
		name        string
		channelName string
		wantErr     bool
	}{
		{name: "valid simple", channelName: "general", wantErr: false},
		{name: "valid underscore", channelName: "team_1", wantErr: false},
		{name: "valid hyphen", channelName: "dev-ops", wantErr: false},
		{name: "invalid empty", channelName: "", wantErr: true},
		{name: "invalid traversal", channelName: "../etc", wantErr: true},
		{name: "invalid windows traversal", channelName: "..\\etc", wantErr: true},
		{name: "invalid slash", channelName: "a/b", wantErr: true},
		{name: "invalid absolute", channelName: "/tmp/test", wantErr: true},
		{name: "invalid uppercase", channelName: "General", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChannelName(tt.channelName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateChannelName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

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

func TestSkipMessage(t *testing.T) {
	botMention := "<@B999>"
	channels := &Channels{authorID: "U123"}
	client := &socketmode.Client{}

	tests := []struct {
		name string
		p    *slackevents.MessageEvent
		want bool
	}{
		{
			name: "skip bot mention",
			p: &slackevents.MessageEvent{
				Text: "<@B999> hello",
				User: "U123",
			},
			want: true,
		},
		{
			name: "skip not author",
			p: &slackevents.MessageEvent{
				Text: "hello",
				User: "U999",
			},
			want: true,
		},
		{
			name: "skip subtype not file_share",
			p: &slackevents.MessageEvent{
				Text:    "hello",
				User:    "U123",
				SubType: "message_changed",
			},
			want: true,
		},
		{
			name: "skip empty text when not file_share",
			p: &slackevents.MessageEvent{
				Text:    "   ",
				User:    "U123",
				SubType: "",
			},
			want: true,
		},
		{
			name: "allow file_share with empty text",
			p: &slackevents.MessageEvent{
				Text:    "   ",
				User:    "U123",
				SubType: "file_share",
			},
			want: false,
		},
		{
			name: "allow normal message",
			p: &slackevents.MessageEvent{
				Text: "hello",
				User: "U123",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skipMessage(tt.p, botMention, client, channels); got != tt.want {
				t.Fatalf("skipMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
