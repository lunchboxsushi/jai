package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current focus and context",
	Long: `Show the current working context including epic, task, and recent activity.

Examples:
  jai status              # Show current context and status`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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

	// Get current context
	currentCtx := ctxManager.Get()

	fmt.Println("=== JAI Status ===")
	fmt.Println()

	// Show current context
	fmt.Println("Current Context:")
	if currentCtx.EpicKey == "" && currentCtx.TaskKey == "" {
		fmt.Println("  No context set")
	} else {
		if currentCtx.EpicKey != "" {
			fmt.Printf("  Epic: %s\n", currentCtx.EpicKey)
		}
		if currentCtx.TaskKey != "" {
			fmt.Printf("  Task: %s\n", currentCtx.TaskKey)
		}
		fmt.Printf("  Last Updated: %s\n", currentCtx.Updated.Format(time.RFC3339))
	}

	fmt.Println()

	// Show recent tickets if context is set
	if currentCtx.EpicKey != "" {
		fmt.Println("Recent Tickets:")
		if err := showRecentTickets(dataDir, currentCtx.EpicKey); err != nil {
			fmt.Printf("  Error loading tickets: %v\n", err)
		}
	}

	fmt.Println()

	// Show configuration status
	fmt.Println("Configuration:")
	showConfigStatus()

	return nil
}

// showRecentTickets shows recent tickets in the current epic
func showRecentTickets(dataDir, epicKey string) error {
	parser := markdown.NewParser(dataDir)
	epicFilePath := parser.GetEpicFilePath(epicKey)

	// Check if epic file exists
	if _, err := os.Stat(epicFilePath); os.IsNotExist(err) {
		fmt.Printf("  Epic file not found: %s\n", epicKey)
		return nil
	}

	// Parse epic file
	mdFile, err := parser.ParseFile(epicFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse epic file: %w", err)
	}

	// Show tickets
	if len(mdFile.Tickets) == 0 {
		fmt.Println("  No tickets found")
		return nil
	}

	// Show last 5 tickets
	count := 0
	for i := len(mdFile.Tickets) - 1; i >= 0 && count < 5; i-- {
		ticket := mdFile.Tickets[i]
		status := "draft"
		if ticket.Key != "" {
			status = "created"
		}
		fmt.Printf("  %s: %s [%s] (%s)\n", ticket.Type, ticket.Title, ticket.Key, status)
		count++
	}

	return nil
}

// showConfigStatus shows the status of configuration
func showConfigStatus() {
	// Check Jira config
	jiraURL := viper.GetString("jira.url")
	jiraUser := viper.GetString("jira.username")
	jiraToken := os.Getenv("JAI_JIRA_TOKEN")
	jiraProject := viper.GetString("jira.project")

	if jiraURL != "" && jiraUser != "" && jiraToken != "" {
		fmt.Printf("  Jira: ✓ Connected to %s (Project: %s)\n", jiraURL, jiraProject)
	} else {
		fmt.Println("  Jira: ✗ Not configured")
		if jiraURL == "" {
			fmt.Println("    - URL not set")
		}
		if jiraUser == "" {
			fmt.Println("    - Username not set")
		}
		if jiraToken == "" {
			fmt.Println("    - Token not set (set JAI_JIRA_TOKEN environment variable)")
		}
	}

	// Check AI config
	aiProvider := viper.GetString("ai.provider")
	aiKey := os.Getenv("JAI_AI_TOKEN")
	aiModel := viper.GetString("ai.model")

	if aiKey != "" {
		if aiProvider == "" {
			aiProvider = "openai"
		}
		if aiModel == "" {
			aiModel = "gpt-3.5-turbo"
		}
		fmt.Printf("  AI: ✓ %s (%s)\n", aiProvider, aiModel)
	} else {
		fmt.Println("  AI: ✗ Not configured")
		fmt.Println("    - API key not set (set JAI_AI_TOKEN environment variable)")
	}

	// Check data directory
	dataDir := viper.GetString("general.data_dir")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share", "jai")
	}
	fmt.Printf("  Data Directory: %s\n", dataDir)
}
