package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dastrobu/mail-mcp/internal/jxa"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed scripts/create_mailbox.js
var createMailboxScript string

// CreateMailboxInput defines input parameters for create_mailbox tool
type CreateMailboxInput struct {
	Account     string   `json:"account" jsonschema:"Name of the email account" long:"account" description:"Name of the email account"`
	MailboxPath []string `json:"mailboxPath" jsonschema:"Path of the mailbox to create. The last element is the new mailbox's name; any preceding elements are the parent path and must already exist (e.g. ['Travel'] or ['WineGeex', 'Receipts']). Names are case-sensitive." long:"mailbox-path" description:"Mailbox path to create (can be specified multiple times for nested mailboxes); the last element is the new mailbox"`
	DryRun      bool     `json:"dryRun,omitempty" jsonschema:"If true, reports whether the mailbox exists or would be created, without creating it" long:"dry-run" description:"Report without creating anything"`
}

// RegisterCreateMailbox registers the create_mailbox tool with the MCP server
func RegisterCreateMailbox(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "create_mailbox",
			Description: "Creates a mailbox (folder) in an account, optionally nested under an existing mailbox. Idempotent: if it already exists the tool reports 'exists' and changes nothing. Supports dryRun.",
			InputSchema: GenerateSchema[CreateMailboxInput](),
			Annotations: &mcp.ToolAnnotations{
				Title:           "Create Mailbox",
				ReadOnlyHint:    false,
				IdempotentHint:  true,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
		},
		HandleCreateMailbox,
	)
}

func HandleCreateMailbox(ctx context.Context, request *mcp.CallToolRequest, input CreateMailboxInput) (*mcp.CallToolResult, any, error) {
	if input.Account == "" {
		return nil, nil, fmt.Errorf("account is required")
	}
	if err := validateMailboxPath(input.MailboxPath); err != nil {
		return nil, nil, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal input for JXA: %w", err)
	}

	data, err := jxa.Execute(ctx, createMailboxScript, string(inputJSON))
	if err != nil {
		return nil, nil, err
	}

	return nil, data, nil
}

// validateMailboxPath rejects empty paths and blank or whitespace-only names.
func validateMailboxPath(path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("mailboxPath is required (the last element is the mailbox to create)")
	}
	for i, p := range path {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("mailboxPath element %d is blank", i)
		}
		if strings.TrimSpace(p) != p {
			return fmt.Errorf("mailboxPath element %q has leading or trailing whitespace", p)
		}
	}
	return nil
}
