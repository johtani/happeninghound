package client

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type stubFileContextGetter struct {
	failByURL map[string]error
}

func (s stubFileContextGetter) GetFileContext(_ context.Context, downloadURL string, writer io.Writer) error {
	if err, ok := s.failByURL[downloadURL]; ok {
		return err
	}
	if _, err := io.WriteString(writer, "ok"); err != nil {
		return err
	}
	return nil
}

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

func TestResolveMakeMDParams(t *testing.T) {
	tests := []struct {
		name        string
		ev          slack.SlashCommand
		wantChannel string
		wantSince   bool
		wantErr     bool
	}{
		{
			name: "no args uses current channel",
			ev: slack.SlashCommand{
				ChannelName: "general",
			},
			wantChannel: "general",
			wantSince:   false,
			wantErr:     false,
		},
		{
			name: "single period arg",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "30d",
			},
			wantChannel: "general",
			wantSince:   true,
			wantErr:     false,
		},
		{
			name: "single channel arg",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "dev-team.jsonl",
			},
			wantChannel: "dev-team",
			wantSince:   false,
			wantErr:     false,
		},
		{
			name: "channel and period",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "dev-team 7d",
			},
			wantChannel: "dev-team",
			wantSince:   true,
			wantErr:     false,
		},
		{
			name: "too many args",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "a b c",
			},
			wantErr: true,
		},
		{
			name: "invalid second arg",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "dev-team 12h",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, since, err := resolveMakeMDParams(tt.ev)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveMakeMDParams() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ch != tt.wantChannel {
				t.Fatalf("resolveMakeMDParams() channel = %q, want %q", ch, tt.wantChannel)
			}
			if (since != nil) != tt.wantSince {
				t.Fatalf("resolveMakeMDParams() since nil=%v, wantSince %v", since == nil, tt.wantSince)
			}
		})
	}
}

func TestResolveMakeHTMLParams(t *testing.T) {
	tests := []struct {
		name         string
		ev           slack.SlashCommand
		wantChannel  string
		wantSince    bool
		wantErr      bool
		wantErrUsage bool
	}{
		{
			name: "no args uses current channel",
			ev: slack.SlashCommand{
				ChannelName: "general",
			},
			wantChannel: "general",
			wantSince:   false,
			wantErr:     false,
		},
		{
			name: "single period arg",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "30d",
			},
			wantChannel: "general",
			wantSince:   true,
			wantErr:     false,
		},
		{
			name: "single channel arg",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "dev-team.jsonl",
			},
			wantChannel: "dev-team",
			wantSince:   false,
			wantErr:     false,
		},
		{
			name: "channel and period",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "dev-team 7d",
			},
			wantChannel: "dev-team",
			wantSince:   true,
			wantErr:     false,
		},
		{
			name: "too many args",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "a b c",
			},
			wantErr:      true,
			wantErrUsage: true,
		},
		{
			name: "invalid second arg includes usage",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "dev-team 12h",
			},
			wantErr:      true,
			wantErrUsage: true,
		},
		{
			name: "invalid period value includes usage",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "0d",
			},
			wantErr:      true,
			wantErrUsage: true,
		},
		{
			name: "single invalid period-like arg includes usage",
			ev: slack.SlashCommand{
				ChannelName: "general",
				Text:        "12h",
			},
			wantErr:      true,
			wantErrUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, since, err := resolveMakeHTMLParams(tt.ev)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveMakeHTMLParams() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantErrUsage && (err == nil || !strings.Contains(err.Error(), "usage: /make-html [channel] [period] or /make-html [period]")) {
					t.Fatalf("resolveMakeHTMLParams() expected usage in error, got: %v", err)
				}
				return
			}
			if ch != tt.wantChannel {
				t.Fatalf("resolveMakeHTMLParams() channel = %q, want %q", ch, tt.wantChannel)
			}
			if (since != nil) != tt.wantSince {
				t.Fatalf("resolveMakeHTMLParams() since nil=%v, wantSince %v", since == nil, tt.wantSince)
			}
		})
	}
}

func TestParseRelativePeriod(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantFound bool
		wantErr   bool
	}{
		{name: "valid", raw: "30d", wantFound: true, wantErr: false},
		{name: "valid upper", raw: "7D", wantFound: true, wantErr: false},
		{name: "empty", raw: "", wantFound: false, wantErr: false},
		{name: "wrong suffix", raw: "30h", wantFound: false, wantErr: false},
		{name: "zero day", raw: "0d", wantFound: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			since, found, err := parseRelativePeriod(tt.raw)
			if found != tt.wantFound {
				t.Fatalf("parseRelativePeriod() found = %v, want %v", found, tt.wantFound)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRelativePeriod() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantFound && !tt.wantErr {
				if since == nil {
					t.Fatalf("parseRelativePeriod() since is nil")
				}
				diff := time.Since(*since)
				if diff < 0 || diff > 31*24*time.Hour {
					t.Fatalf("parseRelativePeriod() unexpected since: %v", *since)
				}
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

func TestBuildShowFilesMessage_IncludesUpdatedAtAndCount(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(path.Join(baseDir, "zeta.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile(zeta) error = %v", err)
	}
	if err := os.WriteFile(path.Join(baseDir, "alpha.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile(alpha) error = %v", err)
	}
	if err := os.WriteFile(path.Join(baseDir, "notes.txt"), []byte("ignore"), 0644); err != nil {
		t.Fatalf("WriteFile(notes) error = %v", err)
	}
	if err := os.MkdirAll(path.Join(baseDir, HtmlDir), os.ModePerm); err != nil {
		t.Fatalf("MkdirAll(html) error = %v", err)
	}
	if err := os.WriteFile(path.Join(baseDir, HtmlDir, "alpha.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("WriteFile(alpha.html) error = %v", err)
	}

	alphaTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	zetaTime := time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(path.Join(baseDir, "alpha.jsonl"), alphaTime, alphaTime); err != nil {
		t.Fatalf("Chtimes(alpha) error = %v", err)
	}
	if err := os.Chtimes(path.Join(baseDir, "zeta.jsonl"), zetaTime, zetaTime); err != nil {
		t.Fatalf("Chtimes(zeta) error = %v", err)
	}

	msg, err := buildShowFilesMessage(baseDir)
	if err != nil {
		t.Fatalf("buildShowFilesMessage() error = %v", err)
	}
	alphaExpected := alphaTime.In(time.Local).Format(showFilesTimeLayout)
	zetaExpected := zetaTime.In(time.Local).Format(showFilesTimeLayout)

	if !strings.Contains(msg, "Files are ...") {
		t.Fatalf("message missing header: %q", msg)
	}
	if !strings.Contains(msg, ":o: alpha.jsonl (updated: "+alphaExpected+")") {
		t.Fatalf("message missing alpha line: %q", msg)
	}
	if !strings.Contains(msg, ":x: zeta.jsonl (updated: "+zetaExpected+")") {
		t.Fatalf("message missing zeta line: %q", msg)
	}
	if !strings.Contains(msg, "count: 2/2") {
		t.Fatalf("message missing count: %q", msg)
	}
	if strings.Contains(msg, "notes.txt") {
		t.Fatalf("message should not include non-jsonl file: %q", msg)
	}
	if strings.Index(msg, "alpha.jsonl") > strings.Index(msg, "zeta.jsonl") {
		t.Fatalf("message should be sorted by file name: %q", msg)
	}
}

func TestBuildShowFilesMessage_ReturnsErrorWhenBaseDirMissing(t *testing.T) {
	_, err := buildShowFilesMessage(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatalf("buildShowFilesMessage() error = nil, want non-nil")
	}
}

func TestExecuteCommand_ShowFilesErrorIncludesMessage(t *testing.T) {
	msg := executeCommand(
		context.Background(),
		slack.SlashCommand{Command: "/show-files"},
		nil,
		nil,
		filepath.Join(t.TempDir(), "missing"),
		nil,
	)
	if !strings.Contains(msg, "Files are ...") {
		t.Fatalf("executeCommand() missing header: %q", msg)
	}
	if !strings.Contains(msg, "Error:") {
		t.Fatalf("executeCommand() missing error line: %q", msg)
	}
}

func TestParseRelativePeriodPatternOnlyD(t *testing.T) {
	_, found, err := parseRelativePeriod("15w")
	if err != nil {
		t.Fatalf("parseRelativePeriod() unexpected err: %v", err)
	}
	if found {
		t.Fatalf("parseRelativePeriod() found = true, want false")
	}

	_, found, err = parseRelativePeriod(" 15d ")
	if err != nil {
		t.Fatalf("parseRelativePeriod() unexpected err: %v", err)
	}
	if !found {
		t.Fatalf("parseRelativePeriod() found = false, want true")
	}

	_, found, err = parseRelativePeriod(strings.Repeat("1", 100) + "d")
	if !found {
		t.Fatalf("parseRelativePeriod() found = false, want true")
	}
	if err == nil {
		t.Fatalf("parseRelativePeriod() err = nil, want error for overflow")
	}
}

func TestDownloadImageFiles_AllSuccess(t *testing.T) {
	channels := &Channels{basedir: t.TempDir()}
	getter := stubFileContextGetter{}
	uploadCalls := 0
	gdrive := &GDrive{
		createImageFileFn: func(ctx context.Context, name, parent, filepath string) error {
			uploadCalls++
			return nil
		},
	}
	files := []slack.File{
		{URLPrivateDownload: "https://example.com/1", Filetype: "png"},
		{URLPrivateDownload: "https://example.com/2", Filetype: "jpg"},
	}

	got, err := downloadImageFiles(context.Background(), getter, "general", channels, files, "1711670400.000000", gdrive)
	if err != nil {
		t.Fatalf("downloadImageFiles() error = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("downloadImageFiles() len = %d, want 2", len(got))
	}
	if uploadCalls != 2 {
		t.Fatalf("upload calls = %d, want 2", uploadCalls)
	}
}

func TestDownloadImageFiles_PartialFailureKeepsSuccesses(t *testing.T) {
	channels := &Channels{basedir: t.TempDir()}
	getter := stubFileContextGetter{
		failByURL: map[string]error{
			"https://example.com/2": errors.New("download failed"),
		},
	}
	uploadCalls := 0
	gdrive := &GDrive{
		createImageFileFn: func(ctx context.Context, name, parent, filepath string) error {
			uploadCalls++
			return nil
		},
	}
	files := []slack.File{
		{URLPrivateDownload: "https://example.com/1", Filetype: "png"},
		{URLPrivateDownload: "https://example.com/2", Filetype: "jpg"},
	}

	got, err := downloadImageFiles(context.Background(), getter, "general", channels, files, "1711670400.000000", gdrive)
	if err == nil {
		t.Fatal("downloadImageFiles() error = nil, want non-nil")
	}
	if len(got) != 1 {
		t.Fatalf("downloadImageFiles() len = %d, want 1", len(got))
	}
	if uploadCalls != 1 {
		t.Fatalf("upload calls = %d, want 1", uploadCalls)
	}
	if !strings.Contains(err.Error(), "index=1") || !strings.Contains(err.Error(), "stage=download") {
		t.Fatalf("downloadImageFiles() error = %q, want index/stage info", err.Error())
	}
}

func TestDownloadImageFiles_AllFailure(t *testing.T) {
	channels := &Channels{basedir: t.TempDir()}
	getter := stubFileContextGetter{
		failByURL: map[string]error{
			"https://example.com/1": errors.New("download failed 1"),
			"https://example.com/2": errors.New("download failed 2"),
		},
	}
	uploadCalls := 0
	gdrive := &GDrive{
		createImageFileFn: func(ctx context.Context, name, parent, filepath string) error {
			uploadCalls++
			return nil
		},
	}
	files := []slack.File{
		{URLPrivateDownload: "https://example.com/1", Filetype: "png"},
		{URLPrivateDownload: "https://example.com/2", Filetype: "jpg"},
	}

	got, err := downloadImageFiles(context.Background(), getter, "general", channels, files, "1711670400.000000", gdrive)
	if err == nil {
		t.Fatal("downloadImageFiles() error = nil, want non-nil")
	}
	if len(got) != 0 {
		t.Fatalf("downloadImageFiles() len = %d, want 0", len(got))
	}
	if uploadCalls != 0 {
		t.Fatalf("upload calls = %d, want 0", uploadCalls)
	}
	if !strings.Contains(err.Error(), "index=0") || !strings.Contains(err.Error(), "index=1") {
		t.Fatalf("downloadImageFiles() error = %q, want both failed indexes", err.Error())
	}
}
