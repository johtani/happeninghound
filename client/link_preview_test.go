package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestAttachLinkPreviews(t *testing.T) {
	calls := 0
	ch := &Channels{
		previewFetcher: func(_ context.Context, rawURL string) (*LinkPreview, error) {
			calls++
			return &LinkPreview{
				URL:   rawURL,
				Title: "preview-title",
			}, nil
		},
	}

	entries := []Entry{
		{Message: "<https://example.com>"},
		{Message: "<https://example.com|same>"},
		{Message: "text <https://example.com>"},
		{Message: "<https://a.example> <https://b.example>"},
	}

	got := ch.attachLinkPreviews(context.Background(), entries)
	if got[0].Preview == nil || got[0].Preview.Title != "preview-title" {
		t.Fatalf("entry[0] preview not set: %+v", got[0].Preview)
	}
	if got[1].Preview == nil || got[1].Preview.Title != "preview-title" {
		t.Fatalf("entry[1] preview not set: %+v", got[1].Preview)
	}
	if got[2].Preview != nil {
		t.Fatalf("entry[2] preview should be nil: %+v", got[2].Preview)
	}
	if got[3].Preview != nil {
		t.Fatalf("entry[3] preview should be nil: %+v", got[3].Preview)
	}
	if calls != 1 {
		t.Fatalf("preview fetcher calls = %d, want 1", calls)
	}
}

func TestParseLinkPreviewFromHTML(t *testing.T) {
	pageURL, err := url.Parse("https://example.com/posts/1")
	if err != nil {
		t.Fatalf("url parse failed: %v", err)
	}

	html := `
<!doctype html>
<html>
<head>
  <title>Fallback Title</title>
  <meta property="og:title" content="OG Title">
  <meta property="og:description" content="OG Desc">
  <meta property="og:image" content="/images/cover.png">
  <meta property="og:site_name" content="Example Site">
</head>
<body></body>
</html>`

	preview, err := parseLinkPreviewFromHTML(pageURL, strings.NewReader(html))
	if err != nil {
		t.Fatalf("parseLinkPreviewFromHTML() error = %v", err)
	}
	if preview == nil {
		t.Fatal("preview is nil")
	}
	if preview.Title != "OG Title" {
		t.Errorf("Title = %q, want %q", preview.Title, "OG Title")
	}
	if preview.Description != "OG Desc" {
		t.Errorf("Description = %q, want %q", preview.Description, "OG Desc")
	}
	if preview.ImageURL != "https://example.com/images/cover.png" {
		t.Errorf("ImageURL = %q, want %q", preview.ImageURL, "https://example.com/images/cover.png")
	}
	if preview.SiteName != "Example Site" {
		t.Errorf("SiteName = %q, want %q", preview.SiteName, "Example Site")
	}
}

func TestBuildLinkPreview_NoMetadata(t *testing.T) {
	pageURL, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatalf("url parse failed: %v", err)
	}
	got := buildLinkPreview(pageURL, "", map[string]string{})
	if got != nil {
		t.Fatalf("buildLinkPreview() = %+v, want nil", got)
	}
}

func TestValidatePreviewURL(t *testing.T) {
	tests := []struct {
		name      string
		rawURL    string
		wantError bool
	}{
		{name: "https public ip", rawURL: "https://8.8.8.8", wantError: false},
		{name: "http public ip", rawURL: "http://1.1.1.1", wantError: false},
		{name: "unsupported scheme", rawURL: "ftp://example.com", wantError: true},
		{name: "localhost blocked", rawURL: "https://localhost/path", wantError: true},
		{name: "loopback blocked", rawURL: "https://127.0.0.1/path", wantError: true},
		{name: "invalid port blocked", rawURL: "https://1.1.1.1:99999/path", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatalf("url parse failed: %v", err)
			}
			err = validatePreviewURL(context.Background(), parsed)
			if tt.wantError && err == nil {
				t.Fatalf("validatePreviewURL() error = nil, want error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("validatePreviewURL() error = %v, want nil", err)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		port      string
		wantError bool
	}{
		{port: "", wantError: false},
		{port: "443", wantError: false},
		{port: "0", wantError: true},
		{port: "70000", wantError: true},
		{port: "abc", wantError: true},
	}
	for _, tt := range tests {
		err := validatePort(tt.port)
		if tt.wantError && err == nil {
			t.Fatalf("validatePort(%q) error=nil, want error", tt.port)
		}
		if !tt.wantError && err != nil {
			t.Fatalf("validatePort(%q) error=%v, want nil", tt.port, err)
		}
	}
}

func TestEntry_LinkURLs(t *testing.T) {
	e := Entry{Message: "a <https://example.com|ex> b <https://example.org>"}
	got := e.LinkURLs()
	want := []string{"https://example.com", "https://example.org"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("LinkURLs() = %v, want %v", got, want)
	}
}
