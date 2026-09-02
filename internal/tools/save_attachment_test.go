package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDirectory(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{"", "", true},
		{"relative/dir", "", true},
		{"~/Documents/Travel", filepath.Join(home, "Documents", "Travel"), false},
		{"~", home, false},
		{"/tmp/x/../y", "/tmp/y", false},
	}
	for _, c := range cases {
		got, err := resolveDirectory(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%q: got %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestSaveAttachmentRejectsBadInputBeforeJXA(t *testing.T) {
	base := SaveAttachmentInput{Account: "A", MailboxPath: []string{"INBOX"}, MessageID: 1, Directory: "/tmp"}

	bad := base
	bad.MessageID = 0
	if _, _, err := HandleSaveAttachment(context.Background(), nil, bad); err == nil {
		t.Error("expected error for messageId 0")
	}

	bad = base
	bad.Directory = "not/absolute"
	if _, _, err := HandleSaveAttachment(context.Background(), nil, bad); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Errorf("expected absolute-path error, got %v", err)
	}

	bad = base
	bad.AttachmentID = "x"
	bad.AttachmentName = "y"
	if _, _, err := HandleSaveAttachment(context.Background(), nil, bad); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected not-both error, got %v", err)
	}
}
