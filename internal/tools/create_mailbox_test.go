package tools

import (
	"context"
	"strings"
	"testing"
)

func TestValidateMailboxPath(t *testing.T) {
	if err := validateMailboxPath(nil); err == nil {
		t.Error("expected error for empty path")
	}
	if err := validateMailboxPath([]string{"Travel", ""}); err == nil {
		t.Error("expected error for blank element")
	}
	if err := validateMailboxPath([]string{" Travel"}); err == nil || !strings.Contains(err.Error(), "whitespace") {
		t.Errorf("expected whitespace error, got %v", err)
	}
	if err := validateMailboxPath([]string{"WineGeex", "Receipts"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateMailboxRejectsBadInputBeforeJXA(t *testing.T) {
	if _, _, err := HandleCreateMailbox(context.Background(), nil, CreateMailboxInput{MailboxPath: []string{"X"}}); err == nil {
		t.Error("expected error for missing account")
	}
	if _, _, err := HandleCreateMailbox(context.Background(), nil, CreateMailboxInput{Account: "A"}); err == nil {
		t.Error("expected error for missing mailboxPath")
	}
}
