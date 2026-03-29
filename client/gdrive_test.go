package client

import (
	"context"
	"testing"

	"google.golang.org/api/drive/v3"
)

func TestGDrive_htmlCreateParentID(t *testing.T) {
	g := GDrive{
		targetDir: &drive.File{Id: "target-dir-id"},
		htmlDir:   &drive.File{Id: "html-dir-id"},
	}

	if got := g.htmlCreateParentID(); got != "html-dir-id" {
		t.Fatalf("htmlCreateParentID() = %q, want %q", got, "html-dir-id")
	}
}

func TestGDrive_CreateImageFile_NilImageDir(t *testing.T) {
	g := GDrive{
		imageDir: nil,
	}
	err := g.CreateImageFile(context.Background(), "test.jpg", "channel", "/tmp/test.jpg")
	if err == nil {
		t.Fatal("expected error when imageDir is nil, got nil")
	}
}

func TestGDrive_UploadFile_NilTargetDir(t *testing.T) {
	g := GDrive{
		targetDir: nil,
	}
	err := g.UploadFile(context.Background(), "test.txt", "/tmp/test.txt")
	if err == nil {
		t.Fatal("expected error when targetDir is nil, got nil")
	}
}

func TestGDrive_UploadHtmlFile_NilHtmlDir(t *testing.T) {
	g := GDrive{
		htmlDir: nil,
	}
	err := g.UploadHtmlFile(context.Background(), "index.html", "/tmp/index.html")
	if err == nil {
		t.Fatal("expected error when htmlDir is nil, got nil")
	}
}
