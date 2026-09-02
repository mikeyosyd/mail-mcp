package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dastrobu/mail-mcp/internal/jxa"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed scripts/save_attachment.js
var saveAttachmentScript string

// SaveAttachmentInput defines input parameters for save_attachment tool
type SaveAttachmentInput struct {
	Account        string   `json:"account" jsonschema:"Name of the email account" long:"account" description:"Name of the email account"`
	MailboxPath    []string `json:"mailboxPath" jsonschema:"Mailbox path array containing the message (e.g., ['INBOX']). Note: Mailbox names are case-sensitive." long:"mailbox-path" description:"Mailbox path (can be specified multiple times for nested mailboxes). Note: Mailbox names are case-sensitive."`
	MessageID      int      `json:"messageId" jsonschema:"ID of the message, as returned by find_messages or get_message_content" long:"message-id" description:"ID of the message"`
	Directory      string   `json:"directory" jsonschema:"Directory to save into. Absolute, or starting with ~ for the home directory. Created if missing." long:"directory" description:"Directory to save into (absolute or ~-relative). Created if missing."`
	AttachmentID   string   `json:"attachmentId,omitempty" jsonschema:"Save only the attachment with this ID (from get_message_content). Omit, with attachmentName, to save all." long:"attachment-id" description:"Save only the attachment with this ID"`
	AttachmentName string   `json:"attachmentName,omitempty" jsonschema:"Save only the attachment with this exact file name. Omit, with attachmentId, to save all." long:"attachment-name" description:"Save only the attachment with this exact name"`
	Overwrite      bool     `json:"overwrite,omitempty" jsonschema:"Overwrite an existing file of the same name (default: false, existing files are skipped)" long:"overwrite" description:"Overwrite existing files (default: skip)"`
	DryRun         bool     `json:"dryRun,omitempty" jsonschema:"If true, reports which attachments would be saved and where, but writes nothing" long:"dry-run" description:"Report what would be saved without writing anything"`
}

// RegisterSaveAttachment registers the save_attachment tool with the MCP server
func RegisterSaveAttachment(srv *mcp.Server) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "save_attachment",
			Description: "Saves a message's attachments to a directory on disk. Saves all attachments unless attachmentId or attachmentName selects one. Existing files are skipped unless overwrite is set; attachments not yet downloaded by Mail.app are skipped and reported. Supports dryRun.",
			InputSchema: GenerateSchema[SaveAttachmentInput](),
			Annotations: &mcp.ToolAnnotations{
				Title:           "Save Attachment",
				ReadOnlyHint:    false,
				IdempotentHint:  true,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
		},
		HandleSaveAttachment,
	)
}

func HandleSaveAttachment(ctx context.Context, request *mcp.CallToolRequest, input SaveAttachmentInput) (*mcp.CallToolResult, any, error) {
	if input.Account == "" {
		return nil, nil, fmt.Errorf("account is required")
	}
	if len(input.MailboxPath) == 0 {
		return nil, nil, fmt.Errorf("mailboxPath is required")
	}
	if input.MessageID <= 0 {
		return nil, nil, fmt.Errorf("messageId must be a positive integer")
	}
	if input.AttachmentID != "" && input.AttachmentName != "" {
		return nil, nil, fmt.Errorf("specify attachmentId or attachmentName, not both")
	}

	dir, err := resolveDirectory(input.Directory)
	if err != nil {
		return nil, nil, err
	}
	input.Directory = dir

	if !input.DryRun {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("failed to create directory %q: %w", dir, err)
		}
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal input for JXA: %w", err)
	}

	data, err := jxa.Execute(ctx, saveAttachmentScript, string(inputJSON))
	if err != nil {
		return nil, nil, err
	}

	return nil, data, nil
}

// resolveDirectory expands a leading ~ and requires the result to be absolute,
// so an attachment can never be written relative to the server's working
// directory.
func resolveDirectory(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("directory is required")
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("directory must be absolute or start with ~ (got %q)", dir)
	}
	return filepath.Clean(dir), nil
}
