package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLinkPreviewCache_PersistAcrossInstances(t *testing.T) {
	baseDir := t.TempDir()
	cache, err := newLinkPreviewCache(baseDir, time.Hour, 10)
	if err != nil {
		t.Fatalf("newLinkPreviewCache() error = %v", err)
	}

	now := time.Now().UTC()
	preview := &LinkPreview{
		URL:   "https://example.com",
		Title: "title",
	}
	cache.Set("https://example.com", preview, now)
	if err := cache.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := newLinkPreviewCache(baseDir, time.Hour, 10)
	if err != nil {
		t.Fatalf("newLinkPreviewCache(reload) error = %v", err)
	}
	got, hit, _ := reloaded.Get("https://example.com", time.Now().UTC())
	if !hit {
		t.Fatal("Get() hit = false, want true")
	}
	if got == nil || got.Title != "title" {
		t.Fatalf("Get() preview = %+v, want title=title", got)
	}
}

func TestLinkPreviewCache_TTLExpired(t *testing.T) {
	baseDir := t.TempDir()
	cache, err := newLinkPreviewCache(baseDir, time.Hour, 10)
	if err != nil {
		t.Fatalf("newLinkPreviewCache() error = %v", err)
	}
	now := time.Now().UTC()
	cache.Set("https://expired.example", &LinkPreview{URL: "https://expired.example", Title: "expired"}, now)

	_, hit, changed := cache.Get("https://expired.example", now.Add(2*time.Hour))
	if hit {
		t.Fatal("Get() hit = true, want false")
	}
	if !changed {
		t.Fatal("Get() changed = false, want true")
	}
}

func TestLinkPreviewCache_PruneByMaxEntries(t *testing.T) {
	baseDir := t.TempDir()
	cache, err := newLinkPreviewCache(baseDir, 24*time.Hour, 2)
	if err != nil {
		t.Fatalf("newLinkPreviewCache() error = %v", err)
	}
	base := time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC)
	cache.Set("https://a.example", &LinkPreview{URL: "https://a.example", Title: "a"}, base)
	cache.Set("https://b.example", &LinkPreview{URL: "https://b.example", Title: "b"}, base.Add(1*time.Minute))
	cache.Set("https://c.example", &LinkPreview{URL: "https://c.example", Title: "c"}, base.Add(2*time.Minute))

	if len(cache.entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(cache.entries))
	}
	if _, ok := cache.entries["https://a.example"]; ok {
		t.Fatal("oldest entry should be pruned")
	}
}

func TestLinkPreviewCache_RecoversCorruptedFile(t *testing.T) {
	baseDir := t.TempDir()
	cacheDir := filepath.Join(baseDir, "cache")
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(cacheDir, linkPreviewCacheFileName)
	if err := os.WriteFile(filePath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cache, err := newLinkPreviewCache(baseDir, time.Hour, 10)
	if err != nil {
		t.Fatalf("newLinkPreviewCache() error = %v", err)
	}
	if len(cache.entries) != 0 {
		t.Fatalf("entries len = %d, want 0", len(cache.entries))
	}

	matches, err := filepath.Glob(filePath + ".broken.*")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("broken cache file was not renamed")
	}
}

func TestAttachLinkPreviews_UsesPersistentCache(t *testing.T) {
	baseDir := t.TempDir()

	cache1, err := newLinkPreviewCache(baseDir, 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("newLinkPreviewCache() error = %v", err)
	}

	calls1 := 0
	ch1 := &Channels{
		previewFetcher: func(_ context.Context, rawURL string) (*LinkPreview, error) {
			calls1++
			return &LinkPreview{URL: rawURL, Title: "cached"}, nil
		},
		previewCache: cache1,
	}
	entries := []Entry{{Message: "<https://example.com>"}}
	first := ch1.attachLinkPreviews(context.Background(), entries)
	if calls1 != 1 {
		t.Fatalf("first fetch calls = %d, want 1", calls1)
	}
	if first[0].Preview == nil || first[0].Preview.Title != "cached" {
		t.Fatalf("first preview = %+v, want cached", first[0].Preview)
	}

	cache2, err := newLinkPreviewCache(baseDir, 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("newLinkPreviewCache(reload) error = %v", err)
	}

	calls2 := 0
	ch2 := &Channels{
		previewFetcher: func(_ context.Context, rawURL string) (*LinkPreview, error) {
			calls2++
			return &LinkPreview{URL: rawURL, Title: "fetched-again"}, nil
		},
		previewCache: cache2,
	}
	second := ch2.attachLinkPreviews(context.Background(), entries)
	if calls2 != 0 {
		t.Fatalf("second fetch calls = %d, want 0", calls2)
	}
	if second[0].Preview == nil || second[0].Preview.Title != "cached" {
		t.Fatalf("second preview = %+v, want cached", second[0].Preview)
	}
}
