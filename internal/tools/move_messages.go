package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/dastrobu/mail-mcp/internal/jxa"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed scripts/move_messages.js
var moveMessagesScript string

// maxBulkMessages caps the number of messages a single move/delete call will
// touch. Each message is one AppleEvent round trip, so this keeps a call well
// inside typical MCP client timeouts; callers page through larger sets.
const maxBulkMessages = 500

// MoveMessagesInput defines input parameters for move_messages tool
type MoveMessagesInput struct {
	Account           string   `json:"account" jsonschema:"Name of the email account" long:"account" description:"Name of the email account"`
	MailboxPath       []string `json:"mailboxPath" jsonschema:"Source mailbox path array (e.g., ['INBOX'] or ['INBOX', 'GitHub']). Note: Mailbox names are case-sensitive." long:"mailbox-path" description:"Source mailbox path (can be specified multiple times for nested mailboxes). Note: Mailbox names are case-sensitive."`
	TargetMailboxPath []string `json:"targetMailboxPath" jsonschema:"Destination mailbox path array (e.g., ['Archive'] or ['Archive', '2026']). Note: Mailbox names are case-sensitive." long:"target-mailbox-path" description:"Destination mailbox path (can be specified multiple times for nested mailboxes). Note: Mailbox names are case-sensitive."`
	MessageIds        []int    `json:"messageIds" jsonschema:"IDs of the messages to move, as returned by find_messages (1-500 per call)" long:"message-id" description:"ID of a message to move (can be specified multiple times, 1-500 per call)"`
	DryRun            bool     `json:"dryRun,omitempty" jsonschema:"If true, resolves mailboxes and looks up every message but moves nothing; returns what would be moved" long:"dry-run" description:"Resolve and report what would be moved without moving anything"`
}

// RegisterMoveMessages registers the move_messages tool with the MCP server
func RegisterMoveMessages(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "move_messages",
			Description: "Moves messages by ID from one mailbox to another within the same account. Use find_messages to obtain IDs first. Supports dryRun to preview. Maximum 500 messages per call.",
			InputSchema: GenerateSchema[MoveMessagesInput](),
			Annotations: &mcp.ToolAnnotations{
				Title:           "Move Messages",
				ReadOnlyHint:    false,
				IdempotentHint:  true,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
		},
		HandleMoveMessages,
	)
}

func HandleMoveMessages(ctx context.Context, request *mcp.CallToolRequest, input MoveMessagesInput) (*mcp.CallToolResult, any, error) {
	if input.Account == "" {
		return nil, nil, fmt.Errorf("account is required")
	}
	if len(input.MailboxPath) == 0 {
		return nil, nil, fmt.Errorf("mailboxPath is required")
	}
	if len(input.TargetMailboxPath) == 0 {
		return nil, nil, fmt.Errorf("targetMailboxPath is required")
	}
	if samePath(input.MailboxPath, input.TargetMailboxPath) {
		return nil, nil, fmt.Errorf("targetMailboxPath must differ from mailboxPath")
	}
	if err := validateMessageIds(input.MessageIds); err != nil {
		return nil, nil, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal input for JXA: %w", err)
	}

	data, err := jxa.Execute(ctx, moveMessagesScript, string(inputJSON))
	if err != nil {
		return nil, nil, err
	}

	return nil, data, nil
}

// validateMessageIds enforces the shared rules for bulk message tools.
func validateMessageIds(ids []int) error {
	if len(ids) == 0 {
		return fmt.Errorf("messageIds is required (at least one message ID)")
	}
	if len(ids) > maxBulkMessages {
		return fmt.Errorf("messageIds must contain at most %d IDs per call (got %d); page through larger sets", maxBulkMessages, len(ids))
	}
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("messageIds must all be positive (got %d)", id)
		}
	}
	return nil
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
