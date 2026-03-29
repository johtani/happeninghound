package client

import (
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
