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
  jai task --no-enrich        # Skip AI enrichment
  jai task --no-create        # Skip Jira ticket creation`,
	RunE: runTask,
}

var (
	noEnrich bool
	noCreate bool
)

func init() {
	taskCmd.Flags().BoolVar(&noEnrich, "no-enrich", false, "Skip AI enrichment")
	taskCmd.Flags().BoolVar(&noCreate, "no-create", false, "Skip Jira ticket creation")
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

	// Check if we have an epic context
	if !ctxManager.HasEpic() {
		return fmt.Errorf("no epic context set. Use 'jai epic <key|title>' first")
	}

	// Get current context
	currentCtx := ctxManager.Get()
	epicKey := currentCtx.EpicKey

	// Initialize parser
	parser := markdown.NewParser(dataDir)
	epicFilePath := parser.GetEpicFilePath(epicKey)

	// Ensure epic file exists
	if err := parser.EnsureFileExists(epicFilePath); err != nil {
		return fmt.Errorf("failed to create epic file: %w", err)
	}

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
		EpicKey:    epicKey,
		Created:    time.Now(),
		Updated:    time.Now(),
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

	// Add task to epic file
	if err := addTaskToEpicFile(parser, epicFilePath, task); err != nil {
		return fmt.Errorf("failed to add task to epic file: %w", err)
	}

	fmt.Printf("Task added to epic %s\n", epicKey)

	// Create Jira ticket if enabled
	if !noCreate {
		fmt.Println("Creating Jira ticket...")
		if err := createJiraTicket(task); err != nil {
			fmt.Printf("Warning: Failed to create Jira ticket: %v\n", err)
		} else {
			fmt.Printf("Jira ticket created: %s\n", task.Key)
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
	template := `# New Task

Describe your task here. Be specific about what needs to be done, why it's important, and any relevant context.

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

	if aiConfig.AI.APIKey == "" {
		return nil, fmt.Errorf("no AI API key configured (set JAI_AI_TOKEN environment variable)")
	}

	if aiConfig.AI.Model == "" {
		aiConfig.AI.Model = "gpt-3.5-turbo" // Default model
	}

	if aiConfig.AI.MaxTokens == 0 {
		aiConfig.AI.MaxTokens = 500 // Default max tokens
	}

	// Create AI service
	aiService := ai.NewService(aiConfig)

	// Create enrichment request
	req := &types.EnrichmentRequest{
		RawContent: task.RawContent,
		Type:       task.Type,
		Context:    *ctx,
	}

	// Enrich the task
	return aiService.EnrichTicket(req)
}

// addTaskToEpicFile adds a task to the epic markdown file
func addTaskToEpicFile(parser *markdown.Parser, epicFilePath string, task *types.Ticket) error {
	// Parse existing file
	mdFile, err := parser.ParseFile(epicFilePath)
	if err != nil {
		// File might not exist or be empty, start fresh
		mdFile = &types.MarkdownFile{
			Path:    epicFilePath,
			Tickets: []types.Ticket{},
		}
	}

	// Add the new task
	mdFile.Tickets = append(mdFile.Tickets, *task)

	// Write back to file
	return parser.WriteFile(epicFilePath, mdFile.Tickets)
}

// createJiraTicket creates a Jira ticket for the task
func createJiraTicket(task *types.Ticket) error {
	// Get Jira config
	jiraConfig := &types.Config{
		Jira: struct {
			URL      string `yaml:"url" json:"url"`
			Username string `yaml:"username" json:"username"`
			Token    string `yaml:"token" json:"token"`
			Project  string `yaml:"project" json:"project"`
		}{
			URL:      viper.GetString("jira.url"),
			Username: viper.GetString("jira.username"),
			Token:    os.Getenv("JAI_JIRA_TOKEN"),
			Project:  viper.GetString("jira.project"),
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
