package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var subtaskCmd = &cobra.Command{
	Use:   "subtask",
	Short: "Add a new sub-task under the current task",
	Long: `Add a new sub-task under the current task context. Opens an editor for drafting,
then enriches the content with AI, and optionally creates a Jira ticket.

Examples:
  jai subtask                    # Create new subtask under current task
  jai subtask --no-enrich        # Skip AI enrichment
  jai subtask --no-create        # Skip Jira ticket creation`,
	RunE: runSubtask,
}

func init() {
	subtaskCmd.Flags().BoolVar(&noEnrich, "no-enrich", false, "Skip AI enrichment")
	subtaskCmd.Flags().BoolVar(&noCreate, "no-create", false, "Skip Jira ticket creation")
	rootCmd.AddCommand(subtaskCmd)
}

func runSubtask(cmd *cobra.Command, args []string) error {
	// Get data directory from config
	dataDir := viper.GetString("general.data_dir")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share", "jai")
	}

	// Initialize context manager
	ctxManager := context.NewManager(dataDir)
	if err := ctxManager.Load(); err != nil {
		return fmt.Errorf("failed to load context: %w", err)
	}

	// Check if we have task context (epic context is optional for subtasks)
	if !ctxManager.HasTask() {
		return fmt.Errorf("no task context set. Use 'jai focus <task>' first")
	}

	// Get current context
	currentCtx := ctxManager.Get()
	taskKey := currentCtx.TaskKey
	epicKey := currentCtx.EpicKey // Optional, may be empty

	// Initialize parser
	parser := markdown.NewParser(dataDir)

	// Determine file path - if we have epic context, use epic file, otherwise use task file
	var filePath string
	if epicKey != "" {
		filePath = parser.GetEpicFilePath(epicKey)
	} else {
		// Create a task-specific file path
		filePath = parser.GetTaskFilePath(taskKey)
	}

	// Ensure file exists
	if err := parser.EnsureFileExists(filePath); err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Open editor for subtask drafting
	rawContent, err := openEditorForSubtask()
	if err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	if strings.TrimSpace(rawContent) == "" {
		fmt.Println("No content provided, subtask creation cancelled")
		return nil
	}

	// Create subtask ticket
	subtask := &types.Ticket{
		Type:       types.TicketTypeSubtask,
		Title:      extractTitleFromContent(rawContent),
		RawContent: rawContent,
		EpicKey:    epicKey,
		ParentKey:  taskKey,
		Created:    time.Now(),
		Updated:    time.Now(),
	}

	// Enrich with AI if enabled
	if !noEnrich {
		fmt.Println("Enriching subtask with AI...")
		enriched, err := enrichTask(subtask, currentCtx)
		if err != nil {
			fmt.Printf("Warning: AI enrichment failed: %v\n", err)
		} else {
			subtask.Enriched = enriched.Description
			subtask.Title = enriched.Title
			subtask.Description = enriched.Description
			if len(enriched.Labels) > 0 {
				subtask.Labels = enriched.Labels
			}
			if enriched.Priority != "" {
				subtask.Priority = enriched.Priority
			}
		}
	}

	// Review before creating if enabled
	if viper.GetBool("general.review_before_create") && !noCreate {
		if err := reviewSubtaskBeforeCreate(subtask, filePath); err != nil {
			return fmt.Errorf("review failed: %w", err)
		}
	}

	// Add subtask to file
	if err := addSubtaskToFile(parser, filePath, subtask); err != nil {
		return fmt.Errorf("failed to add subtask to file: %w", err)
	}

	// Show success message based on context
	if epicKey != "" {
		fmt.Printf("Subtask added to task %s in epic %s\n", taskKey, epicKey)
	} else {
		fmt.Printf("Subtask added to task %s\n", taskKey)
	}

	// Create Jira ticket if enabled
	if !noCreate {
		fmt.Println("Creating Jira ticket...")
		if err := createJiraTicket(subtask); err != nil {
			fmt.Printf("Warning: Failed to create Jira ticket: %v\n", err)
		} else {
			fmt.Printf("Jira ticket created: %s\n", subtask.Key)
		}
	}

	return nil
}

// openEditorForSubtask opens an editor for drafting a subtask
func openEditorForSubtask() (string, error) {
	// Get editor from config or environment
	editor := viper.GetString("general.default_editor")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Default fallback
		}
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "jai-subtask-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write template to temp file
	template := `## Overview
Brief description of what this sub-task aims to achieve.

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Notes
Any additional notes or context...
`
	if _, err := tmpFile.WriteString(template); err != nil {
		return "", fmt.Errorf("failed to write template: %w", err)
	}
	tmpFile.Close()

	// Open editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run editor: %w", err)
	}

	// Read content back
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}

	return string(content), nil
}

// addSubtaskToFile adds a subtask to the markdown file
func addSubtaskToFile(parser *markdown.Parser, filePath string, subtask *types.Ticket) error {
	// Parse existing file
	mdFile, err := parser.ParseFile(filePath)
	if err != nil {
		// File might not exist or be empty, start fresh
		mdFile = &types.MarkdownFile{
			Path:    filePath,
			Tickets: []types.Ticket{},
		}
	}

	// Add the new subtask
	mdFile.Tickets = append(mdFile.Tickets, *subtask)

	// Write back to file
	return parser.WriteFile(filePath, mdFile.Tickets)
}

// reviewSubtaskBeforeCreate opens the markdown file for review and asks for confirmation
func reviewSubtaskBeforeCreate(subtask *types.Ticket, filePath string) error {
	// Get editor from config or environment
	editor := viper.GetString("general.default_editor")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Default fallback
		}
	}

	// Create a temporary file with the current content
	tmpFile, err := os.CreateTemp("", "jai-review-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Create review content
	reviewContent := fmt.Sprintf(`# Review Subtask Before Creating Jira Ticket

File: %s

## Subtask Content to be Created:
%s

---
Review the subtask above. The subtask will be added to the epic file and a Jira ticket will be created.
Save and exit to proceed, or delete all content to cancel.
`, filePath, formatSubtaskForReview(subtask))

	if _, err := tmpFile.WriteString(reviewContent); err != nil {
		return fmt.Errorf("failed to write review content: %w", err)
	}
	tmpFile.Close()

	// Open editor for review
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	// Read content back
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to read temp file: %w", err)
	}

	// Check if user cancelled (deleted all content)
	if strings.TrimSpace(string(content)) == "" {
		return fmt.Errorf("subtask creation cancelled by user")
	}

	// Ask for final confirmation
	fmt.Print("Proceed with creating Jira ticket? (y/n): ")
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(strings.TrimSpace(response)) != "y" && strings.ToLower(strings.TrimSpace(response)) != "yes" {
		return fmt.Errorf("subtask creation cancelled by user")
	}

	return nil
}

// formatSubtaskForReview formats a subtask for display in the review
func formatSubtaskForReview(subtask *types.Ticket) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("### Title\n%s", subtask.Title))

	if subtask.Enriched != "" {
		parts = append(parts, fmt.Sprintf("### Content\n%s", subtask.Enriched))
	} else if subtask.RawContent != "" {
		parts = append(parts, fmt.Sprintf("### Content\n%s", subtask.RawContent))
	}

	if subtask.ParentKey != "" {
		parts = append(parts, fmt.Sprintf("### Parent Task\n%s", subtask.ParentKey))
	}

	return strings.Join(parts, "\n\n")
}
