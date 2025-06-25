package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lunchboxsushi/jai/internal/ai"
	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/jira"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Add a new task under the current epic",
	Long: `Add a new task under the current epic context. Opens an editor for drafting,
then enriches the content with AI, and optionally creates a Jira ticket.

Examples:
  jai task                    # Create new task under current epic
  jai task --orphan           # Create parentless task (no epic)
  jai task --no-enrich        # Skip AI enrichment
  jai task --no-create        # Skip Jira ticket creation`,
	RunE: runTask,
}

var (
	noEnrich bool
	noCreate bool
	orphan   bool
)

func init() {
	taskCmd.Flags().BoolVar(&noEnrich, "no-enrich", false, "Skip AI enrichment")
	taskCmd.Flags().BoolVar(&noCreate, "no-create", false, "Skip Jira ticket creation")
	taskCmd.Flags().BoolVarP(&orphan, "orphan", "o", false, "Create task without parent epic")
	rootCmd.AddCommand(taskCmd)
}

func runTask(cmd *cobra.Command, args []string) error {
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

	var epicKey string
	var currentCtx *types.Context

	// Check epic context only if not creating an orphan task
	if !orphan {
		// Check if we have an epic context
		if !ctxManager.HasEpic() {
			return fmt.Errorf("no epic context set. Use 'jai epic <key|title>' first, or use --orphan to create a parentless task")
		}
		// Get current context
		currentCtx = ctxManager.Get()
		epicKey = currentCtx.EpicKey
	} else {
		// For orphan tasks, create a minimal context
		currentCtx = &types.Context{}
	}

	// Initialize parser
	parser := markdown.NewParser(dataDir)

	// Open editor for task drafting
	rawContent, err := openEditorForTask()
	if err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	if strings.TrimSpace(rawContent) == "" {
		fmt.Println("No content provided, task creation cancelled")
		return nil
	}

	// Create task ticket
	task := &types.Ticket{
		Type:       types.TicketTypeTask,
		Title:      extractTitleFromContent(rawContent),
		RawContent: rawContent,
		EpicKey:    epicKey, // Will be empty for orphan tasks
		Created:    time.Now(),
		Updated:    time.Now(),
		Assignee:   viper.GetString("jira.username"),
	}

	// Enrich with AI if enabled
	if !noEnrich {
		fmt.Println("Enriching task with AI...")
		enriched, err := enrichTask(task, currentCtx)
		if err != nil {
			fmt.Printf("Warning: AI enrichment failed: %v\n", err)
		} else {
			task.Enriched = enriched.Description
			task.Title = enriched.Title
			task.Description = enriched.Description
			if len(enriched.Labels) > 0 {
				task.Labels = enriched.Labels
			}
			if enriched.Priority != "" {
				task.Priority = enriched.Priority
			}
		}
	}

	// Review before creating if enabled
	if viper.GetBool("general.review_before_create") && !noCreate {
		if err := reviewTaskBeforeCreate(task, parser.GetTaskFilePath("")); err != nil {
			return fmt.Errorf("review failed: %w", err)
		}
	}

	// Create separate task file instead of adding to epic
	taskFilePath := parser.GetTaskFilePath("") // Will be renamed after Jira creation
	if err := createTaskFile(parser, taskFilePath, task); err != nil {
		return fmt.Errorf("failed to create task file: %w", err)
	}

	fmt.Printf("Task created in separate file\n")

	// Create Jira ticket if enabled
	if !noCreate {
		fmt.Println("Creating Jira ticket...")
		if err := createJiraTicket(task); err != nil {
			fmt.Printf("Warning: Failed to create Jira ticket: %v\n", err)
		} else {
			fmt.Printf("Jira ticket created: %s\n", task.Key)

			// Update the task file with the real Jira key and rename if needed
			if err := updateTaskWithJiraKey(parser, taskFilePath, task); err != nil {
				fmt.Printf("Warning: Failed to update task with Jira key: %v\n", err)
			}
		}
	} else {
		// Even if not creating Jira ticket, rename the file to the correct format
		if err := renameTaskFile(taskFilePath, task); err != nil {
			fmt.Printf("Warning: Failed to rename task file: %v\n", err)
		} else {
			// Clean up the old file if it still exists and is empty
			if info, err := os.Stat(taskFilePath); err == nil && info.Size() == 0 {
				_ = os.Remove(taskFilePath)
			}
		}
	}

	// Set focus to the newly created task
	if task.Key != "" {
		// If we have a Jira key, use it
		if err := ctxManager.SetTask(task.Key, task.ID); err != nil {
			fmt.Printf("Warning: Failed to set task focus: %v\n", err)
		} else {
			fmt.Printf("Focused on task: %s [%s]\n", task.Title, task.Key)
		}
	} else {
		// If no Jira key, handle focus differently for orphan vs epic tasks
		if task.EpicKey != "" {
			// Task has an epic, update the epic context
			if err := ctxManager.SetEpic(task.EpicKey, ""); err != nil {
				fmt.Printf("Warning: Failed to set epic focus: %v\n", err)
			} else {
				fmt.Printf("Task created under epic: %s\n", task.EpicKey)
			}
		} else {
			// Orphan task without Jira key - set task context with generated key or title
			taskKey := task.Key
			if taskKey == "" {
				taskKey = generateTaskKey(task.Title)
			}
			if err := ctxManager.SetTask(taskKey, task.ID); err != nil {
				fmt.Printf("Warning: Failed to set task focus: %v\n", err)
			} else {
				fmt.Printf("Orphan task created and focused: %s\n", task.Title)
			}
		}
	}

	return nil
}

// openEditorForTask opens an editor for drafting a task
func openEditorForTask() (string, error) {
	// Get editor from config or environment
	editor := viper.GetString("general.default_editor")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Default fallback
		}
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "jai-task-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write template to temp file
	template := `## Overview
Brief description of what this task aims to achieve.

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

// extractTitleFromContent extracts a title from the raw content
func extractTitleFromContent(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") {
			// Use first non-empty, non-header line as title
			if len(line) > 100 {
				return line[:97] + "..."
			}
			return line
		}
	}
	return "Untitled Task"
}

// enrichTask enriches a task using AI
func enrichTask(task *types.Ticket, ctx *types.Context) (*types.EnrichmentResponse, error) {
	fmt.Printf("Starting AI enrichment for task: %s\n", task.Title)

	// Get AI config
	aiConfig := &types.Config{
		AI: struct {
			Provider  string `yaml:"provider" json:"provider"`
			APIKey    string `yaml:"api_key" json:"api_key"`
			Model     string `yaml:"model" json:"model"`
			MaxTokens int    `yaml:"max_tokens" json:"max_tokens"`
		}{
			Provider:  viper.GetString("ai.provider"),
			APIKey:    os.Getenv("JAI_AI_TOKEN"),
			Model:     viper.GetString("ai.model"),
			MaxTokens: viper.GetInt("ai.max_tokens"),
		},
	}

	fmt.Printf("AI Config - Provider: %s, Model: %s, MaxTokens: %d\n",
		aiConfig.AI.Provider, aiConfig.AI.Model, aiConfig.AI.MaxTokens)

	if aiConfig.AI.APIKey == "" {
		fmt.Println("ERROR: No AI API key configured (JAI_AI_TOKEN environment variable not set)")
		return nil, fmt.Errorf("no AI API key configured (set JAI_AI_TOKEN environment variable)")
	}

	if aiConfig.AI.Model == "" {
		aiConfig.AI.Model = "gpt-3.5-turbo" // Default model
		fmt.Printf("Using default model: %s\n", aiConfig.AI.Model)
	}

	if aiConfig.AI.MaxTokens == 0 {
		aiConfig.AI.MaxTokens = 500 // Default max tokens
		fmt.Printf("Using default max tokens: %d\n", aiConfig.AI.MaxTokens)
	}

	fmt.Println("Creating AI service...")
	// Create AI service
	aiService := ai.NewService(aiConfig)

	// Create enrichment request
	req := &types.EnrichmentRequest{
		RawContent: task.RawContent,
		Type:       task.Type,
		Context:    *ctx,
	}

	fmt.Printf("Enrichment request - Type: %s, RawContent length: %d, EpicKey: %s, TaskKey: %s\n",
		req.Type, len(req.RawContent), req.Context.EpicKey, req.Context.TaskKey)

	fmt.Println("Calling AI service to enrich ticket...")
	// Enrich the task
	resp, err := aiService.EnrichTicket(req)
	if err != nil {
		fmt.Printf("ERROR: AI enrichment failed: %v\n", err)
		return nil, err
	}

	fmt.Printf("AI enrichment successful - Title: %s, Description length: %d, Labels: %v, Priority: %s\n",
		resp.Title, len(resp.Description), resp.Labels, resp.Priority)

	return resp, nil
}

// createTaskFile creates a separate task file with epic reference
func createTaskFile(parser *markdown.Parser, taskFilePath string, task *types.Ticket) error {
	// Ensure directory exists
	dir := filepath.Dir(taskFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate markdown content with epic reference
	content := generateTaskMarkdown(task)

	return os.WriteFile(taskFilePath, []byte(content), 0644)
}

// generateTaskMarkdown generates markdown content for a task with epic reference
func generateTaskMarkdown(task *types.Ticket) string {
	var lines []string

	// Add epic reference at the top only if task has an epic
	if task.EpicKey != "" {
		lines = append(lines, fmt.Sprintf("**Epic:** [%s](%s.md)", task.EpicKey, task.EpicKey))
		lines = append(lines, "")
	}

	// Add task header
	header := fmt.Sprintf("## task: %s", task.Title)
	if task.Key != "" {
		header = fmt.Sprintf("## task: %s [%s]", task.Title, task.Key)
	}
	lines = append(lines, header)
	lines = append(lines, "")

	// Add raw content
	if task.RawContent != "" {
		lines = append(lines, task.RawContent)
		lines = append(lines, "")
	}

	// Add enriched content if available
	if task.Enriched != "" {
		lines = append(lines, "---")
		lines = append(lines, "*Enriched:*")
		lines = append(lines, task.Enriched)
		lines = append(lines, "")
	}

	// Add metadata section
	lines = append(lines, "---")
	lines = append(lines, "*Metadata:*")
	if task.Key != "" {
		lines = append(lines, fmt.Sprintf("- Key: %s", task.Key))
	}
	if task.Status != "" {
		lines = append(lines, fmt.Sprintf("- Status: %s", task.Status))
	}
	if task.Priority != "" {
		lines = append(lines, fmt.Sprintf("- Priority: %s", task.Priority))
	}
	if task.EpicKey != "" {
		lines = append(lines, fmt.Sprintf("- ParentKey: %s", task.EpicKey))
	}
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// createJiraTicket creates a Jira ticket for the task
func createJiraTicket(task *types.Ticket) error {
	// Get Jira config
	jiraConfig := &types.Config{
		Jira: struct {
			URL           string `yaml:"url" json:"url"`
			Username      string `yaml:"username" json:"username"`
			Token         string `yaml:"token" json:"token"`
			Project       string `yaml:"project" json:"project"`
			EpicLinkField string `yaml:"epic_link_field" json:"epic_link_field"`
		}{
			URL:           viper.GetString("jira.url"),
			Username:      viper.GetString("jira.username"),
			Token:         os.Getenv("JAI_JIRA_TOKEN"),
			Project:       viper.GetString("jira.project"),
			EpicLinkField: viper.GetString("jira.epic_link_field"),
		},
	}

	if jiraConfig.Jira.URL == "" || jiraConfig.Jira.Username == "" || jiraConfig.Jira.Token == "" {
		return fmt.Errorf("Jira configuration incomplete (check URL, username, and JAI_JIRA_TOKEN environment variable)")
	}

	// Create Jira client using our internal wrapper
	jiraClient, err := jira.NewClient(jiraConfig)
	if err != nil {
		return fmt.Errorf("failed to create Jira client: %w", err)
	}

	// Create the ticket using our wrapper
	createdTicket, err := jiraClient.CreateTicket(task)
	if err != nil {
		return fmt.Errorf("failed to create Jira ticket: %w", err)
	}

	// Update the task with the created data
	*task = *createdTicket

	return nil
}

// reviewTaskBeforeCreate opens the task file for review and asks for confirmation
func reviewTaskBeforeCreate(task *types.Ticket, taskFilePath string) error {
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
	reviewContent := fmt.Sprintf(`# Review Task Before Creating Jira Ticket

File: %s

## Task Content to be Created:
%s

---
Review the task above. The task will be created as a separate file and a Jira ticket will be created.
Save and exit to proceed, or delete all content to cancel.
`, taskFilePath, formatTaskForReview(task))

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
		return fmt.Errorf("task creation cancelled by user")
	}

	// Ask for final confirmation
	fmt.Print("Proceed with creating Jira ticket? (y/n): ")
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(strings.TrimSpace(response)) != "y" && strings.ToLower(strings.TrimSpace(response)) != "yes" {
		return fmt.Errorf("task creation cancelled by user")
	}

	return nil
}

// formatTaskForReview formats a task for display in the review
func formatTaskForReview(task *types.Ticket) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("### Title\n%s", task.Title))

	if task.Enriched != "" {
		parts = append(parts, fmt.Sprintf("### Content\n%s", task.Enriched))
	} else if task.RawContent != "" {
		parts = append(parts, fmt.Sprintf("### Content\n%s", task.RawContent))
	}

	return strings.Join(parts, "\n\n")
}

// updateTaskWithJiraKey updates the task with the Jira key and renames the file
func updateTaskWithJiraKey(parser *markdown.Parser, taskFilePath string, task *types.Ticket) error {
	// Parse existing file to get the task data
	mdFile, err := parser.ParseFile(taskFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse task file: %w", err)
	}

	// Find and update the task with the real key
	for i, t := range mdFile.Tickets {
		if t.Type == types.TicketTypeTask && t.EpicKey == task.EpicKey && t.Title == task.Title {
			mdFile.Tickets[i].Key = task.Key
			// Update the task reference for regeneration
			*task = mdFile.Tickets[i]
			break
		}
	}

	// Regenerate the markdown content with the new key
	content := generateTaskMarkdown(task)

	// Write the updated content back to the file
	if err := os.WriteFile(taskFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	// Rename the file to the correct format
	if err := renameTaskFile(taskFilePath, task); err != nil {
		return fmt.Errorf("failed to rename task file: %w", err)
	}

	return nil
}

// renameTaskFile renames the task file to the correct SRE-####-{ticket title} format
func renameTaskFile(currentPath string, task *types.Ticket) error {
	// Create the new filename in the correct format
	// Convert title to filename-safe format
	safeTitle := strings.ReplaceAll(task.Title, " ", "-")
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

	// Use task key if available, otherwise generate one
	taskKey := task.Key
	if taskKey == "" {
		taskKey = generateTaskKey(task.Title)
	}

	newFilename := fmt.Sprintf("%s-%s.md", taskKey, safeTitle)

	// Get the directory of the current file
	dir := filepath.Dir(currentPath)
	newPath := filepath.Join(dir, newFilename)

	// Check if the new file already exists
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("task file already exists: %s", newPath)
	}

	// Rename the file
	if err := os.Rename(currentPath, newPath); err != nil {
		return fmt.Errorf("failed to rename task file from %s to %s: %w", currentPath, newPath, err)
	}

	fmt.Printf("Task file renamed to: %s\n", newFilename)
	return nil
}

// generateTaskKey generates a Jira-style key for a task
func generateTaskKey(title string) string {
	// Extract project prefix from config or use default
	project := viper.GetString("jira.project")
	if project == "" {
		project = "SRE" // Default to SRE for now
	}

	// Generate a simple key based on title
	words := strings.Fields(title)
	if len(words) > 0 {
		// Use first word as part of the key
		key := strings.ToUpper(words[0])
		if len(key) > 3 {
			key = key[:3]
		}
		return fmt.Sprintf("%s-%d", key, time.Now().Unix()%10000)
	}

	return fmt.Sprintf("%s-%d", project, time.Now().Unix()%10000)
}
