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

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Create a new epic",
	Long: `Create a new epic. Opens an editor for drafting,
then enriches the content with AI, and optionally creates a Jira ticket.

Examples:
  jai epic                    # Create new epic with template
  jai epic --no-enrich       # Skip AI enrichment
  jai epic --no-create       # Skip Jira ticket creation`,
	RunE: runEpic,
}

func init() {
	epicCmd.Flags().BoolVar(&noEnrich, "no-enrich", false, "Skip AI enrichment")
	epicCmd.Flags().BoolVar(&noCreate, "no-create", false, "Skip Jira ticket creation")
	rootCmd.AddCommand(epicCmd)
}

func runEpic(cmd *cobra.Command, args []string) error {
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

	// Create new epic
	return createNewEpic(ctxManager, dataDir)
}

func createNewEpic(ctxManager *context.Manager, dataDir string) error {
	// Open editor for epic drafting
	rawContent, err := openEditorForEpic()
	if err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	if strings.TrimSpace(rawContent) == "" {
		fmt.Println("No content provided, epic creation cancelled")
		return nil
	}

	// Extract title from content
	title := extractTitleFromContent(rawContent)

	// Create epic ticket
	epic := &types.Ticket{
		Type:       types.TicketTypeEpic,
		Title:      title,
		RawContent: rawContent,
		Created:    time.Now(),
		Updated:    time.Now(),
	}

	// Enrich with AI if enabled
	if !noEnrich {
		fmt.Println("Enriching epic with AI...")
		enriched, err := enrichEpic(epic)
		if err != nil {
			fmt.Printf("Warning: AI enrichment failed: %v\n", err)
		} else {
			epic.Enriched = enriched.Description
			epic.Title = enriched.Title
			epic.Description = enriched.Description
			if len(enriched.Labels) > 0 {
				epic.Labels = enriched.Labels
			}
			if enriched.Priority != "" {
				epic.Priority = enriched.Priority
			}
		}
	}

	// Generate temporary epic key for file creation
	tempEpicKey := generateEpicKey(epic.Title)

	// Initialize parser and create epic file
	parser := markdown.NewParser(dataDir)
	epicFilePath := parser.GetEpicFilePath(tempEpicKey)

	// Ensure epic file exists
	if err := parser.EnsureFileExists(epicFilePath); err != nil {
		return fmt.Errorf("failed to create epic file: %w", err)
	}

	// Add epic to file
	if err := addEpicToFile(parser, epicFilePath, epic); err != nil {
		return fmt.Errorf("failed to add epic to file: %w", err)
	}

	// Rename the file to the correct format before review
	renamedFilePath, err := renameEpicFile(epicFilePath, tempEpicKey, []types.Ticket{*epic})
	if err != nil {
		fmt.Printf("Warning: Failed to rename epic file: %v\n", err)
	} else {
		// Update the file path to the new name for the review
		epicFilePath = renamedFilePath
	}

	// Review before creating if enabled
	if viper.GetBool("general.review_before_create") && !noCreate {
		if err := reviewEpicBeforeCreate(epic, epicFilePath); err != nil {
			return fmt.Errorf("review failed: %w", err)
		}
	}

	// Set epic context
	if err := ctxManager.SetEpic(tempEpicKey, ""); err != nil {
		return fmt.Errorf("failed to set epic context: %w", err)
	}

	fmt.Printf("Epic added: %s [%s]\n", epic.Title, tempEpicKey)

	// Create Jira ticket if enabled
	if !noCreate {
		fmt.Println("Creating Jira epic...")
		if err := createJiraEpic(epic); err != nil {
			fmt.Printf("Warning: Failed to create Jira epic: %v\n", err)
		} else {
			fmt.Printf("Jira epic created: %s\n", epic.Key)

			// Update the epic file with the real Jira key
			if err := updateEpicWithJiraKey(parser, epicFilePath, tempEpicKey, epic.Key); err != nil {
				fmt.Printf("Warning: Failed to update epic file with Jira key: %v\n", err)
			} else {
				// Update context with real Jira key
				if err := ctxManager.SetEpic(epic.Key, epic.ID); err != nil {
					fmt.Printf("Warning: Failed to update context with Jira key: %v\n", err)
				} else {
					fmt.Printf("Updated epic context to: %s\n", epic.Key)
				}
			}
		}
	}

	return nil
}

// openEditorForEpic opens an editor for drafting an epic
func openEditorForEpic() (string, error) {
	// Get editor from config or environment
	editor := viper.GetString("general.default_editor")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Default fallback
		}
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "jai-epic-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write template to temp file
	template := `## Overview
Brief description of what this epic aims to achieve.

## Goals
- [ ] Goal 1
- [ ] Goal 2

## Success Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Stakeholders
- Stakeholder 1
- Stakeholder 2

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

// enrichEpic enriches an epic using AI
func enrichEpic(epic *types.Ticket) (*types.EnrichmentResponse, error) {
	fmt.Printf("Starting AI enrichment for epic: %s\n", epic.Title)

	// Get AI config
	aiConfig := &types.Config{
		AI: struct {
			Provider       string `yaml:"provider" json:"provider"`
			APIKey         string `yaml:"api_key" json:"api_key"`
			Model          string `yaml:"model" json:"model"`
			MaxTokens      int    `yaml:"max_tokens" json:"max_tokens"`
			PromptTemplate string `yaml:"prompt_template" json:"prompt_template"`
		}{
			Provider:       viper.GetString("ai.provider"),
			APIKey:         os.Getenv("JAI_AI_TOKEN"),
			Model:          viper.GetString("ai.model"),
			MaxTokens:      viper.GetInt("ai.max_tokens"),
			PromptTemplate: viper.GetString("ai.prompt_template"),
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
		RawContent: epic.RawContent,
		Type:       epic.Type,
		Context:    types.Context{}, // Empty context for epics
	}

	fmt.Printf("Enrichment request - Type: %s, RawContent length: %d\n",
		req.Type, len(req.RawContent))

	fmt.Println("Calling AI service to enrich epic...")
	// Enrich the epic
	resp, err := aiService.EnrichTicket(req)
	if err != nil {
		fmt.Printf("ERROR: AI enrichment failed: %v\n", err)
		return nil, err
	}

	fmt.Printf("AI enrichment successful - Title: %s, Description length: %d, Labels: %v, Priority: %s\n",
		resp.Title, len(resp.Description), resp.Labels, resp.Priority)

	return resp, nil
}

// addEpicToFile adds an epic to the markdown file
func addEpicToFile(parser *markdown.Parser, epicFilePath string, epic *types.Ticket) error {
	// Parse existing file
	mdFile, err := parser.ParseFile(epicFilePath)
	if err != nil {
		// File might not exist or be empty, start fresh
		mdFile = &types.MarkdownFile{
			Path:    epicFilePath,
			Tickets: []types.Ticket{},
		}
	}

	// Add the new epic
	mdFile.Tickets = append(mdFile.Tickets, *epic)

	// Write back to file
	return parser.WriteFile(epicFilePath, mdFile.Tickets)
}

// createJiraEpic creates a Jira epic
func createJiraEpic(epic *types.Ticket) error {
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

	// Create the epic using our wrapper
	createdEpic, err := jiraClient.CreateTicket(epic)
	if err != nil {
		return fmt.Errorf("failed to create Jira epic: %w", err)
	}

	// Update the epic with the created data
	*epic = *createdEpic

	return nil
}

// updateEpicWithJiraKey updates the epic file with the real Jira key
func updateEpicWithJiraKey(parser *markdown.Parser, epicFilePath string, tempKey string, realKey string) error {
	// Parse existing file
	mdFile, err := parser.ParseFile(epicFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse epic file: %w", err)
	}

	// Find and update the epic with the real key
	for i, ticket := range mdFile.Tickets {
		if ticket.Key == tempKey || (ticket.Key == "" && ticket.Title != "") {
			mdFile.Tickets[i].Key = realKey
			break
		}
	}

	// Write back to file
	if err := parser.WriteFile(epicFilePath, mdFile.Tickets); err != nil {
		return fmt.Errorf("failed to write epic file: %w", err)
	}

	// Rename the file to the correct format
	_, err = renameEpicFile(epicFilePath, realKey, mdFile.Tickets)
	if err != nil {
		return fmt.Errorf("failed to rename epic file: %w", err)
	}

	return nil
}

// renameEpicFile renames the epic file to the correct SRE-####-{ticket title} format
func renameEpicFile(currentPath string, epicKey string, tickets []types.Ticket) (string, error) {
	// Find the epic ticket to get its title
	var epicTitle string
	for _, ticket := range tickets {
		if ticket.Key == epicKey {
			epicTitle = ticket.Title
			break
		}
	}

	if epicTitle == "" {
		return "", fmt.Errorf("could not find epic title for key: %s", epicKey)
	}

	// Create the new filename in the correct format
	// Convert title to filename-safe format
	safeTitle := strings.ReplaceAll(epicTitle, " ", "-")
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

	newFilename := fmt.Sprintf("%s-%s.md", epicKey, safeTitle)

	// Get the directory of the current file
	dir := filepath.Dir(currentPath)
	newPath := filepath.Join(dir, newFilename)

	// Check if the new file already exists
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("epic file already exists: %s", newPath)
	}

	// Rename the file
	if err := os.Rename(currentPath, newPath); err != nil {
		return "", fmt.Errorf("failed to rename epic file from %s to %s: %w", currentPath, newPath, err)
	}

	fmt.Printf("Epic file renamed to: %s\n", newFilename)
	return newPath, nil
}

// reviewEpicBeforeCreate opens the epic file for review and asks for confirmation
func reviewEpicBeforeCreate(epic *types.Ticket, epicFilePath string) error {
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
	reviewContent := fmt.Sprintf(`# Review Epic Before Creating Jira Ticket

File: %s

## Epic Content to be Created:
%s

---
Review the epic above. The epic will be added to the file and a Jira epic will be created.
Save and exit to proceed, or delete all content to cancel.
`, epicFilePath, formatEpicForReview(epic))

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
		return fmt.Errorf("epic creation cancelled by user")
	}

	// Ask for final confirmation
	fmt.Print("Proceed with creating Jira epic? (y/n): ")
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(strings.TrimSpace(response)) != "y" && strings.ToLower(strings.TrimSpace(response)) != "yes" {
		return fmt.Errorf("epic creation cancelled by user")
	}

	return nil
}

// formatEpicForReview formats an epic for display in the review
func formatEpicForReview(epic *types.Ticket) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("### Title\n%s", epic.Title))

	if epic.Enriched != "" {
		parts = append(parts, fmt.Sprintf("### Content\n%s", epic.Enriched))
	} else if epic.RawContent != "" {
		parts = append(parts, fmt.Sprintf("### Content\n%s", epic.RawContent))
	}

	return strings.Join(parts, "\n\n")
}

// generateEpicKey generates a Jira-style key from a title
func generateEpicKey(title string) string {
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
