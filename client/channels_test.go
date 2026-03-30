package client

import (
	"html/template"
	"testing"
)

func TestEntry_MessageWithLinkTag(t *testing.T) {
	type fields struct {
		Timestamp   string
		Message     string
		ChannelId   string
		ChannelName string
		Files       []string
	}
	tests := []struct {
		name   string
		fields fields
		want   template.HTML
	}{
		{name: "no link", fields: fields{Message: "no link"}, want: "no link"},
		{name: "one link", fields: fields{Message: "a\u003chttps://example.com/index.html\u003e"}, want: "a<a href=\"https://example.com/index.html\" target=\"_blank\">https://example.com/index.html</a>"},
		{name: "two links", fields: fields{Message: "a\u003chttps://example.com/index.html\u003e b\u003chttps://example.com/index.html\u003e"}, want: "a<a href=\"https://example.com/index.html\" target=\"_blank\">https://example.com/index.html</a> b<a href=\"https://example.com/index.html\" target=\"_blank\">https://example.com/index.html</a>"},
		{name: "><><", fields: fields{Message: "><><"}, want: template.HTML(template.HTMLEscapeString("><><"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Entry{
				Timestamp:   tt.fields.Timestamp,
				Message:     tt.fields.Message,
				ChannelId:   tt.fields.ChannelId,
				ChannelName: tt.fields.ChannelName,
				Files:       tt.fields.Files,
			}
			if got := e.MessageWithLinkTag(); got != tt.want {
				t.Errorf("MessageWithLinkTag() = %v, want %v", got, tt.want)
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
		{name: "valid timestamp 9digits", timestamp: "1633024800.123456789", want: "2021-09-30 18:00:00"},
		{name: "valid timestamp 6digits", timestamp: "1633024800.123456", want: "2021-09-30 18:00:00"},
		{name: "valid timestamp 10digits", timestamp: "1633024800.1234567890", want: "2021-09-30 18:00:00"},
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
