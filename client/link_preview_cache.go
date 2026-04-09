package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	defaultLinkPreviewCacheTTL        = 7 * 24 * time.Hour
	defaultLinkPreviewCacheMaxEntries = 1000
	linkPreviewCacheFileName          = "link_preview_cache.json"
	linkPreviewCacheVersion           = 1
)

type linkPreviewCache struct {
	filePath   string
	ttl        time.Duration
	maxEntries int

	mu      sync.Mutex
	entries map[string]linkPreviewCacheEntry
}

type linkPreviewCacheFile struct {
	Version int                              `json:"version"`
	Entries map[string]linkPreviewCacheEntry `json:"entries"`
}

type linkPreviewCacheEntry struct {
	Preview      LinkPreview `json:"preview"`
	FetchedAt    time.Time   `json:"fetched_at"`
	ExpiresAt    time.Time   `json:"expires_at"`
	LastAccessed time.Time   `json:"last_accessed"`
}

func newLinkPreviewCache(baseDir string, ttl time.Duration, maxEntries int) (*linkPreviewCache, error) {
	if ttl <= 0 {
		ttl = defaultLinkPreviewCacheTTL
	}
	if maxEntries <= 0 {
		maxEntries = defaultLinkPreviewCacheMaxEntries
	}

	c := &linkPreviewCache{
		filePath:   filepath.Join(baseDir, "cache", linkPreviewCacheFileName),
		ttl:        ttl,
		maxEntries: maxEntries,
		entries:    map[string]linkPreviewCacheEntry{},
	}
	if err := c.load(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *linkPreviewCache) Get(rawURL string, now time.Time) (*LinkPreview, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[rawURL]
	if !ok {
		return nil, false, false
	}
	if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
		delete(c.entries, rawURL)
		return nil, false, true
	}
	entry.LastAccessed = now.UTC()
	c.entries[rawURL] = entry

	preview := entry.Preview
	return &preview, true, true
}

func (c *linkPreviewCache) Set(rawURL string, preview *LinkPreview, now time.Time) bool {
	if preview == nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now = now.UTC()
	c.entries[rawURL] = linkPreviewCacheEntry{
		Preview:      *preview,
		FetchedAt:    now,
		ExpiresAt:    now.Add(c.ttl),
		LastAccessed: now,
	}

	changed := true
	if c.pruneToLimitLocked() {
		changed = true
	}
	return changed
}

func (c *linkPreviewCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

func (c *linkPreviewCache) load() error {
	b, err := os.ReadFile(c.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("リンクプレビューキャッシュ読込失敗: %w", err)
	}

	var disk linkPreviewCacheFile
	if err := json.Unmarshal(b, &disk); err != nil {
		c.handleCorruptedCacheFile(err)
		return nil
	}
	if disk.Entries == nil {
		return nil
	}

	now := time.Now().UTC()
	for rawURL, entry := range disk.Entries {
		if entry.FetchedAt.IsZero() {
			entry.FetchedAt = now
		}
		if entry.ExpiresAt.IsZero() {
			entry.ExpiresAt = entry.FetchedAt.Add(c.ttl)
		}
		if entry.LastAccessed.IsZero() {
			entry.LastAccessed = entry.FetchedAt
		}
		if now.After(entry.ExpiresAt) {
			continue
		}
		c.entries[rawURL] = entry
	}

	if c.pruneToLimitLocked() {
		if err := c.saveLocked(); err != nil {
			log.Printf("リンクプレビューキャッシュ保存失敗(読み込み時整理後): %v", err)
		}
	}
	return nil
}

func (c *linkPreviewCache) handleCorruptedCacheFile(parseErr error) {
	brokenPath := fmt.Sprintf("%s.broken.%s", c.filePath, time.Now().UTC().Format("20060102150405"))
	if err := os.Rename(c.filePath, brokenPath); err != nil {
		log.Printf("破損キャッシュの退避失敗: file=%s err=%v", c.filePath, err)
		return
	}
	log.Printf("破損したリンクプレビューキャッシュを退避: src=%s dst=%s err=%v", c.filePath, brokenPath, parseErr)
}

func (c *linkPreviewCache) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(c.filePath), os.ModePerm); err != nil {
		return fmt.Errorf("リンクプレビューキャッシュディレクトリ作成失敗: %w", err)
	}
	disk := linkPreviewCacheFile{
		Version: linkPreviewCacheVersion,
		Entries: c.entries,
	}
	out, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("リンクプレビューキャッシュJSON化失敗: %w", err)
	}

	tmpPath := c.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return fmt.Errorf("リンクプレビューキャッシュ一時保存失敗: %w", err)
	}
	if err := os.Rename(tmpPath, c.filePath); err != nil {
		if removeErr := os.Remove(c.filePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("リンクプレビューキャッシュ置換前の既存ファイル削除失敗: %w", removeErr)
		}
		if retryErr := os.Rename(tmpPath, c.filePath); retryErr != nil {
			return fmt.Errorf("リンクプレビューキャッシュ置換失敗: %w", retryErr)
		}
	}
	return nil
}

func (c *linkPreviewCache) pruneToLimitLocked() bool {
	if c.maxEntries <= 0 || len(c.entries) <= c.maxEntries {
		return false
	}
	type pruneItem struct {
		url string
		at  time.Time
	}
	items := make([]pruneItem, 0, len(c.entries))
	for rawURL, entry := range c.entries {
		at := entry.LastAccessed
		if at.IsZero() {
			at = entry.FetchedAt
		}
		items = append(items, pruneItem{
			url: rawURL,
			at:  at,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].at.Before(items[j].at)
	})

	removeCount := len(c.entries) - c.maxEntries
	for i := 0; i < removeCount; i++ {
		delete(c.entries, items[i].url)
	}
	return true
}
