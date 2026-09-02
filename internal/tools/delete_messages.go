package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/dastrobu/mail-mcp/internal/jxa"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed scripts/delete_messages.js
var deleteMessagesScript string

// DeleteMessagesInput defines input parameters for delete_messages tool
type DeleteMessagesInput struct {
	Account     string   `json:"account" jsonschema:"Name of the email account" long:"account" description:"Name of the email account"`
	MailboxPath []string `json:"mailboxPath" jsonschema:"Mailbox path array containing the messages (e.g., ['INBOX'] or ['INBOX', 'GitHub']). Note: Mailbox names are case-sensitive." long:"mailbox-path" description:"Mailbox path (can be specified multiple times for nested mailboxes). Note: Mailbox names are case-sensitive."`
	MessageIds  []int    `json:"messageIds" jsonschema:"IDs of the messages to delete, as returned by find_messages (1-500 per call)" long:"message-id" description:"ID of a message to delete (can be specified multiple times, 1-500 per call)"`
	DryRun      bool     `json:"dryRun,omitempty" jsonschema:"If true, looks up every message but deletes nothing; returns what would be deleted" long:"dry-run" description:"Resolve and report what would be deleted without deleting anything"`
}

// RegisterDeleteMessages registers the delete_messages tool with the MCP server
func RegisterDeleteMessages(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "delete_messages",
			Description: "Deletes messages by ID. Messages are moved to the account's Trash (Deleted Messages) per Mail.app behaviour and are not permanently removed until the Trash is emptied. Use find_messages to obtain IDs first. Supports dryRun to preview. Maximum 500 messages per call.",
			InputSchema: GenerateSchema[DeleteMessagesInput](),
			Annotations: &mcp.ToolAnnotations{
				Title:           "Delete Messages",
				ReadOnlyHint:    false,
				IdempotentHint:  true,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
			},
		},
		HandleDeleteMessages,
	)
}

func HandleDeleteMessages(ctx context.Context, request *mcp.CallToolRequest, input DeleteMessagesInput) (*mcp.CallToolResult, any, error) {
	if input.Account == "" {
		return nil, nil, fmt.Errorf("account is required")
	}
	if len(input.MailboxPath) == 0 {
		return nil, nil, fmt.Errorf("mailboxPath is required")
	}
	if err := validateMessageIds(input.MessageIds); err != nil {
		return nil, nil, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal input for JXA: %w", err)
	}

	data, err := jxa.Execute(ctx, deleteMessagesScript, string(inputJSON))
	if err != nil {
		return nil, nil, err
	}

	return nil, data, nil
}
