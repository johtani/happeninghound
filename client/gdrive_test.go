package client

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
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

func TestGDrive_UploadHtmlFile_CreateUsesHtmlDirAsParent(t *testing.T) {
	tracer = otel.GetTracerProvider().Tracer("client-test")

	called := false
	var gotParent string
	g := GDrive{
		htmlDir: &drive.File{Id: "html-dir-id"},
		getTargetFileFn: func(ctx context.Context, filename, dirid string) (*drive.File, error) {
			return nil, nil
		},
		createFileFn: func(ctx context.Context, name, parent, filepath string) error {
			called = true
			gotParent = parent
			return nil
		},
	}

	if err := g.UploadHtmlFile(context.Background(), "index.html", "/tmp/index.html"); err != nil {
		t.Fatalf("UploadHtmlFile() error = %v", err)
	}
	if !called {
		t.Fatal("createFile was not called")
	}
	if gotParent != "html-dir-id" {
		t.Fatalf("createFile parent = %q, want %q", gotParent, "html-dir-id")
	}
}

func TestGDrive_UploadHtmlFile_UpdateWhenFileExists(t *testing.T) {
	tracer = otel.GetTracerProvider().Tracer("client-test")

	createCalled := false
	updateCalled := false
	g := GDrive{
		htmlDir: &drive.File{Id: "html-dir-id"},
		getTargetFileFn: func(ctx context.Context, filename, dirid string) (*drive.File, error) {
			return &drive.File{Id: "existing-file-id", Name: filename}, nil
		},
		createFileFn: func(ctx context.Context, name, parent, filepath string) error {
			createCalled = true
			return nil
		},
		updateFileFn: func(ctx context.Context, name, id, filepath string) error {
			updateCalled = true
			if id != "existing-file-id" {
				t.Fatalf("updateFile id = %q, want %q", id, "existing-file-id")
			}
			return nil
		},
	}

	if err := g.UploadHtmlFile(context.Background(), "index.html", "/tmp/index.html"); err != nil {
		t.Fatalf("UploadHtmlFile() error = %v", err)
	}
	if createCalled {
		t.Fatal("createFile should not be called when file exists")
	}
	if !updateCalled {
		t.Fatal("updateFile was not called")
	}
}

func TestGDrive_UploadFile_TargetFileSearchError(t *testing.T) {
	tracer = otel.GetTracerProvider().Tracer("client-test")

	expected := errors.New("drive api temporary failure")
	g := GDrive{
		targetDir: &drive.File{Id: "target-dir-id"},
		getTargetFileFn: func(ctx context.Context, filename, dirid string) (*drive.File, error) {
			return nil, expected
		},
		createFileFn: func(ctx context.Context, name, parent, filepath string) error {
			t.Fatal("createFile should not be called when targetFile search fails")
			return nil
		},
	}

	err := g.UploadFile(context.Background(), "test.txt", "/tmp/test.txt")
	if err == nil {
		t.Fatal("expected error when targetFile search fails, got nil")
	}
}

func TestGDrive_UploadHtmlFile_TargetFileSearchError(t *testing.T) {
	tracer = otel.GetTracerProvider().Tracer("client-test")

	expected := errors.New("drive api temporary failure")
	g := GDrive{
		htmlDir: &drive.File{Id: "html-dir-id"},
		getTargetFileFn: func(ctx context.Context, filename, dirid string) (*drive.File, error) {
			return nil, expected
		},
		createFileFn: func(ctx context.Context, name, parent, filepath string) error {
			t.Fatal("createFile should not be called when targetFile search fails")
			return nil
		},
	}

	err := g.UploadHtmlFile(context.Background(), "index.html", "/tmp/index.html")
	if err == nil {
		t.Fatal("expected error when targetFile search fails, got nil")
	}
}
