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
		Assignee:   viper.GetString("jira.username"),
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
		if err := reviewSubtaskBeforeCreate(subtask, parser.GetTaskFilePath("")); err != nil {
			return fmt.Errorf("review failed: %w", err)
		}
	}

	// Create separate subtask file instead of adding to existing file
	subtaskFilePath := parser.GetTaskFilePath("") // Will be renamed after Jira creation
	if err := createSubtaskFile(parser, subtaskFilePath, subtask); err != nil {
		return fmt.Errorf("failed to create subtask file: %w", err)
	}

	fmt.Printf("Subtask created in separate file\n")

	// Create Jira ticket if enabled
	if !noCreate {
		fmt.Println("Creating Jira ticket...")
		if err := createJiraTicket(subtask); err != nil {
			fmt.Printf("Warning: Failed to create Jira ticket: %v\n", err)
		} else {
			fmt.Printf("Jira ticket created: %s\n", subtask.Key)

			// Update the subtask file with the real Jira key and rename if needed
			if err := updateSubtaskWithJiraKey(parser, subtaskFilePath, subtask); err != nil {
				fmt.Printf("Warning: Failed to update subtask with Jira key: %v\n", err)
			}
		}
	} else {
		// Even if not creating Jira ticket, rename the file to the correct format
		if err := renameSubtaskFile(subtaskFilePath, subtask); err != nil {
			fmt.Printf("Warning: Failed to rename subtask file: %v\n", err)
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

// createSubtaskFile creates a separate subtask file with task/epic references
func createSubtaskFile(parser *markdown.Parser, subtaskFilePath string, subtask *types.Ticket) error {
	// Ensure directory exists
	dir := filepath.Dir(subtaskFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate markdown content with task/epic references
	content := generateSubtaskMarkdown(subtask)

	return os.WriteFile(subtaskFilePath, []byte(content), 0644)
}

// generateSubtaskMarkdown generates markdown content for a subtask with task/epic references
func generateSubtaskMarkdown(subtask *types.Ticket) string {
	var lines []string

	// Add task reference at the top
	if subtask.ParentKey != "" {
		lines = append(lines, fmt.Sprintf("**Task:** [%s](%s.md)", subtask.ParentKey, subtask.ParentKey))
		lines = append(lines, "")
	}

	// Add epic reference if available
	if subtask.EpicKey != "" {
		lines = append(lines, fmt.Sprintf("**Epic:** [%s](%s.md)", subtask.EpicKey, subtask.EpicKey))
		lines = append(lines, "")
	}

	// Add subtask header
	header := fmt.Sprintf("### subtask: %s", subtask.Title)
	if subtask.Key != "" {
		header = fmt.Sprintf("### subtask: %s [%s]", subtask.Title, subtask.Key)
	}
	lines = append(lines, header)
	lines = append(lines, "")

	// Add raw content
	if subtask.RawContent != "" {
		lines = append(lines, subtask.RawContent)
		lines = append(lines, "")
	}

	// Add enriched content if available
	if subtask.Enriched != "" {
		lines = append(lines, "---")
		lines = append(lines, "*Enriched:*")
		lines = append(lines, subtask.Enriched)
		lines = append(lines, "")
	}

	// Add metadata section
	lines = append(lines, "---")
	lines = append(lines, "*Metadata:*")
	if subtask.Key != "" {
		lines = append(lines, fmt.Sprintf("- Key: %s", subtask.Key))
	}
	if subtask.Status != "" {
		lines = append(lines, fmt.Sprintf("- Status: %s", subtask.Status))
	}
	if subtask.Priority != "" {
		lines = append(lines, fmt.Sprintf("- Priority: %s", subtask.Priority))
	}
	if subtask.ParentKey != "" {
		lines = append(lines, fmt.Sprintf("- TaskKey: %s", subtask.ParentKey))
	}
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// updateSubtaskWithJiraKey updates the subtask with the Jira key and renames the file
func updateSubtaskWithJiraKey(parser *markdown.Parser, subtaskFilePath string, subtask *types.Ticket) error {
	// Parse existing file to get the subtask data
	mdFile, err := parser.ParseFile(subtaskFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse subtask file: %w", err)
	}

	// Find and update the subtask with the real key
	for i, s := range mdFile.Tickets {
		if s.Type == types.TicketTypeSubtask && s.ParentKey == subtask.ParentKey && s.Title == subtask.Title {
			mdFile.Tickets[i].Key = subtask.Key
			// Update the subtask reference for regeneration
			*subtask = mdFile.Tickets[i]
			break
		}
	}

	// Regenerate the markdown content with the new key
	content := generateSubtaskMarkdown(subtask)

	// Write the updated content back to the file
	if err := os.WriteFile(subtaskFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write subtask file: %w", err)
	}

	// Rename the file to the correct format
	if err := renameSubtaskFile(subtaskFilePath, subtask); err != nil {
		return fmt.Errorf("failed to rename subtask file: %w", err)
	}

	return nil
}

// renameSubtaskFile renames the subtask file to the correct SRE-####-{ticket title} format
func renameSubtaskFile(currentPath string, subtask *types.Ticket) error {
	// Create the new filename in the correct format
	// Convert title to filename-safe format
	safeTitle := strings.ReplaceAll(subtask.Title, " ", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "/", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "\\", "-")
	safeTitle = strings.ReplaceAll(safeTitle, ":", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "*", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "?", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "\"", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "<", "-")
	safeTitle = strings.ReplaceAll(safeTitle, ">", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "|", "-")

	// Remove any double dashes and trim
	safeTitle = strings.ReplaceAll(safeTitle, "--", "-")
	safeTitle = strings.Trim(safeTitle, "-")

	// Use subtask key if available, otherwise generate one
	subtaskKey := subtask.Key
	if subtaskKey == "" {
		subtaskKey = generateSubtaskKey(subtask.Title)
	}

	newFilename := fmt.Sprintf("%s-%s.md", subtaskKey, safeTitle)

	// Get the directory of the current file
	dir := filepath.Dir(currentPath)
	newPath := filepath.Join(dir, newFilename)

	// Rename the file
	if err := os.Rename(currentPath, newPath); err != nil {
		return fmt.Errorf("failed to rename subtask file: %w", err)
	}

	fmt.Printf("Subtask file renamed to: %s\n", newFilename)
	return nil
}

// generateSubtaskKey generates a key for a subtask
func generateSubtaskKey(title string) string {
	// Generate a simple key based on title
	words := strings.Fields(strings.ToUpper(title))
	if len(words) == 0 {
		return "SUB-001"
	}

	// Take first word and add a number
	prefix := words[0]
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	return fmt.Sprintf("%s-001", prefix)
}

// reviewSubtaskBeforeCreate opens the subtask file for review and asks for confirmation
func reviewSubtaskBeforeCreate(subtask *types.Ticket, subtaskFilePath string) error {
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
Review the subtask above. The subtask will be created as a separate file and a Jira ticket will be created.
Save and exit to proceed, or delete all content to cancel.
`, subtaskFilePath, formatSubtaskForReview(subtask))

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
