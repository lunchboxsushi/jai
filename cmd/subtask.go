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

	// Check if we have both epic and task context
	if !ctxManager.HasEpic() {
		return fmt.Errorf("no epic context set. Use 'jai epic <key|title>' first")
	}
	if !ctxManager.HasTask() {
		return fmt.Errorf("no task context set. Use 'jai focus <task>' first")
	}

	// Get current context
	currentCtx := ctxManager.Get()
	epicKey := currentCtx.EpicKey
	taskKey := currentCtx.TaskKey

	// Initialize parser
	parser := markdown.NewParser(dataDir)
	epicFilePath := parser.GetEpicFilePath(epicKey)

	// Ensure epic file exists
	if err := parser.EnsureFileExists(epicFilePath); err != nil {
		return fmt.Errorf("failed to create epic file: %w", err)
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

	// Add subtask to epic file
	if err := addSubtaskToEpicFile(parser, epicFilePath, subtask); err != nil {
		return fmt.Errorf("failed to add subtask to epic file: %w", err)
	}

	fmt.Printf("Subtask added to task %s in epic %s\n", taskKey, epicKey)

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
	template := `# New Sub-task

Describe your sub-task here. This should be a smaller, more focused piece of work.

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

// addSubtaskToEpicFile adds a subtask to the epic markdown file
func addSubtaskToEpicFile(parser *markdown.Parser, epicFilePath string, subtask *types.Ticket) error {
	// Parse existing file
	mdFile, err := parser.ParseFile(epicFilePath)
	if err != nil {
		// File might not exist or be empty, start fresh
		mdFile = &types.MarkdownFile{
			Path:    epicFilePath,
			Tickets: []types.Ticket{},
		}
	}

	// Add the new subtask
	mdFile.Tickets = append(mdFile.Tickets, *subtask)

	// Write back to file
	return parser.WriteFile(epicFilePath, mdFile.Tickets)
}
