package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateMessageIds(t *testing.T) {
	if err := validateMessageIds(nil); err == nil {
		t.Fatal("expected error for empty IDs")
	}
	if err := validateMessageIds([]int{1, 0}); err == nil {
		t.Fatal("expected error for non-positive ID")
	}
	tooMany := make([]int, maxBulkMessages+1)
	for i := range tooMany {
		tooMany[i] = i + 1
	}
	if err := validateMessageIds(tooMany); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("expected cap error, got %v", err)
	}
	if err := validateMessageIds([]int{1, 2, 3}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// The handlers must reject bad input before ever reaching osascript.
func TestMoveMessagesRejectsSameMailbox(t *testing.T) {
	_, _, err := HandleMoveMessages(context.Background(), nil, MoveMessagesInput{
		Account:           "iCloud",
		MailboxPath:       []string{"INBOX"},
		TargetMailboxPath: []string{"INBOX"},
		MessageIds:        []int{1},
	})
	if err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("expected same-mailbox error, got %v", err)
	}
}

func TestDeleteMessagesRequiresIds(t *testing.T) {
	_, _, err := HandleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		Account:     "iCloud",
		MailboxPath: []string{"INBOX"},
	})
	if err == nil {
		t.Fatal("expected error for missing messageIds")
	}
}

func TestBulkSchemasMarkRequiredFields(t *testing.T) {
	cases := map[string]struct {
		schema   any
		required []string
	}{
		"move":   {GenerateSchema[MoveMessagesInput](), []string{"account", "mailboxPath", "targetMailboxPath", "messageIds"}},
		"delete": {GenerateSchema[DeleteMessagesInput](), []string{"account", "mailboxPath", "messageIds"}},
	}
	for name, tc := range cases {
		raw, err := json.Marshal(tc.schema)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var parsed struct {
			Required   []string       `json:"required"`
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		got := map[string]bool{}
		for _, r := range parsed.Required {
			got[r] = true
		}
		for _, want := range tc.required {
			if !got[want] {
				t.Errorf("%s: %q should be required; required=%v", name, want, parsed.Required)
			}
		}
		if got["dryRun"] {
			t.Errorf("%s: dryRun must be optional", name)
		}
		if _, ok := parsed.Properties["messageIds"]; !ok {
			t.Errorf("%s: messageIds property missing", name)
		}
	}
}
