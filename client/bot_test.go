package client

import (
	"os"
	"path"
	"strings"
	"testing"
)

func TestEnsureCSSFile_CopyWhenNotExist(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.MkdirAll(path.Join(baseDir, HtmlDir), os.ModePerm); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := ensureCSSFile(baseDir, os.Stat); err != nil {
		t.Fatalf("ensureCSSFile() error = %v", err)
	}

	cssPath := path.Join(baseDir, HtmlDir, CSSFile)
	content, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("CSS file is empty")
	}
}

func TestEnsureCSSFile_ReturnsErrorWhenStatErrorIsNotNotExist(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.MkdirAll(path.Join(baseDir, HtmlDir), os.ModePerm); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	err := ensureCSSFile(baseDir, func(_ string) (os.FileInfo, error) {
		return nil, os.ErrPermission
	})
	if err == nil {
		t.Fatalf("ensureCSSFile() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "状態確認に失敗") {
		t.Fatalf("ensureCSSFile() error = %v, want status check failure", err)
	}

	cssPath := path.Join(baseDir, HtmlDir, CSSFile)
	if _, statErr := os.Stat(cssPath); !os.IsNotExist(statErr) {
		t.Fatalf("CSS file should not be created when stat fails: %v", statErr)
	}
}
