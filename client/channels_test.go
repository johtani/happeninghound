package client

import (
	"fmt"
	"html/template"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEntry_MessageWithLinkTag(t *testing.T) {
	type fields struct {
		Timestamp string
		Message   string
		Channel   Channel
		Files     []string
	}
	tests := []struct {
		name   string
		fields fields
		want   template.HTML
	}{
		{name: "no link", fields: fields{Message: "no link"}, want: "no link"},
		{name: "one link", fields: fields{Message: "a\u003chttps://example.com/index.html\u003e"}, want: "a<a href=\"https://example.com/index.html\" target=\"_blank\" rel=\"noopener noreferrer\">https://example.com/index.html</a>"},
		{name: "two links", fields: fields{Message: "a\u003chttps://example.com/index.html\u003e b\u003chttps://example.com/index.html\u003e"}, want: "a<a href=\"https://example.com/index.html\" target=\"_blank\" rel=\"noopener noreferrer\">https://example.com/index.html</a> b<a href=\"https://example.com/index.html\" target=\"_blank\" rel=\"noopener noreferrer\">https://example.com/index.html</a>"},
		{name: "link with label", fields: fields{Message: "\u003chttps://example.com|example\u003e"}, want: "<a href=\"https://example.com\" target=\"_blank\" rel=\"noopener noreferrer\">example</a>"},
		{name: "text escaped with link", fields: fields{Message: "x < y \u003chttps://example.com|z\u003e"}, want: "x &lt; y <a href=\"https://example.com\" target=\"_blank\" rel=\"noopener noreferrer\">z</a>"},
		{name: "label escaped", fields: fields{Message: "\u003chttps://example.com|a&b\u003e"}, want: "<a href=\"https://example.com\" target=\"_blank\" rel=\"noopener noreferrer\">a&amp;b</a>"},
		{name: "><><", fields: fields{Message: "><><"}, want: template.HTML(template.HTMLEscapeString("><><"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Entry{
				Timestamp: tt.fields.Timestamp,
				Message:   tt.fields.Message,
				Channel:   tt.fields.Channel,
				Files:     tt.fields.Files,
			}
			if got := e.MessageWithLinkTag(); got != tt.want {
				t.Errorf("MessageWithLinkTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_IsLinkOnlyMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{name: "single link", message: "<https://example.com>", want: true},
		{name: "single link with label", message: "<https://example.com|example>", want: true},
		{name: "multiple links with spaces", message: " <https://a.example>  <https://b.example|b> ", want: true},
		{name: "text and link", message: "note <https://example.com>", want: false},
		{name: "no link", message: "hello", want: false},
		{name: "empty", message: "   ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Entry{Message: tt.message}
			if got := e.IsLinkOnlyMessage(); got != tt.want {
				t.Errorf("IsLinkOnlyMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_Timestamp2String(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		want      string
	}{
		{name: "valid timestamp 1digit", timestamp: "1633024800.1", want: "2021-09-30 18:00:00"},
		{name: "valid timestamp 8digits", timestamp: "1633024800.12345678", want: "2021-09-30 18:00:00"},
		{name: "valid timestamp 9digits", timestamp: "1633024800.123456789", want: "2021-09-30 18:00:00"},
		{name: "valid timestamp 6digits", timestamp: "1633024800.123456", want: "2021-09-30 18:00:00"},
		{name: "valid timestamp 10digits", timestamp: "1633024800.1234567890", want: "2021-09-30 18:00:00"},
		{name: "invalid fractional timestamp", timestamp: "1633024800.abc", want: ""},
		{name: "invalid timestamp", timestamp: "invalid.timestamp", want: ""},
		{name: "no dot timestamp", timestamp: "1633024800", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Entry{
				Timestamp: tt.timestamp,
			}
			if got := e.Timestamp2String(); got != tt.want {
				t.Errorf("Timestamp2String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEntry(t *testing.T) {
	tests := []struct {
		name    string
		jsonl   string
		want    Entry
		wantErr bool
	}{
		{
			name:  "valid json (new schema)",
			jsonl: `{"timestamp":"1633024800.123456","message":"hello","channel":{"id":"C123","name":"general"},"files":["a.png"]}`,
			want: Entry{
				Timestamp: "1633024800.123456",
				Message:   "hello",
				Channel: Channel{
					ID:   "C123",
					Name: "general",
				},
				Files: []string{"a.png"},
			},
			wantErr: false,
		},
		{
			name:  "valid json (legacy schema)",
			jsonl: `{"timestamp":"1633024800.123456","message":"hello","channel.id":"C999","channel.name":"legacy","files":["a.png"]}`,
			want: Entry{
				Timestamp: "1633024800.123456",
				Message:   "hello",
				Channel: Channel{
					ID:   "C999",
					Name: "legacy",
				},
				Files: []string{"a.png"},
			},
			wantErr: false,
		},
		{
			name:  "both schema prefers new channel object",
			jsonl: `{"timestamp":"1633024800.123456","message":"hello","channel":{"id":"C123","name":"general"},"channel.id":"C999","channel.name":"legacy","files":["a.png"]}`,
			want: Entry{
				Timestamp: "1633024800.123456",
				Message:   "hello",
				Channel: Channel{
					ID:   "C123",
					Name: "general",
				},
				Files: []string{"a.png"},
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			jsonl:   `{"timestamp":"1633024800.123456",`,
			want:    Entry{},
			wantErr: true,
		},
		{
			name:    "empty string",
			jsonl:   ``,
			want:    Entry{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEntry(tt.jsonl)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseEntry() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseEntriesFromJSONL_LongLine(t *testing.T) {
	longMessage := strings.Repeat("a", 70*1024)
	input := fmt.Sprintf("{\"timestamp\":\"1633024800.123456\",\"message\":\"%s\",\"channel\":{\"id\":\"C123\",\"name\":\"general\"},\"files\":[]}\n", longMessage)

	entries, err := parseEntriesFromJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseEntriesFromJSONL() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("parseEntriesFromJSONL() len = %d, want 1", len(entries))
	}
	if entries[0].Message != longMessage {
		t.Errorf("parseEntriesFromJSONL() message length = %d, want %d", len(entries[0].Message), len(longMessage))
	}
}

func TestChannels_safeJoinUnderBase(t *testing.T) {
	baseDir := t.TempDir()
	c := &Channels{basedir: baseDir}

	tests := []struct {
		name    string
		relPath string
		wantErr bool
	}{
		{name: "jsonl file under base", relPath: "general.jsonl", wantErr: false},
		{name: "html file under sub dir", relPath: filepath.Join("html", "general.html"), wantErr: false},
		{name: "parent traversal", relPath: filepath.Join("..", "secret.txt"), wantErr: true},
		{name: "absolute path", relPath: filepath.Join(baseDir, "outside.txt"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.safeJoinUnderBase(tt.relPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("safeJoinUnderBase() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !strings.HasPrefix(got, baseDir) {
				t.Fatalf("safeJoinUnderBase() path = %q, want prefix %q", got, baseDir)
			}
		})
	}
}
