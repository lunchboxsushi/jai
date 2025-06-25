package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current focus and context",
	Long: `Show the current working context including epic, task, and recent activity.

Examples:
  jai status              # Show current context and status
  jai status --config     # Show current context and configuration`,
	RunE: runStatus,
}

var showConfigDetails bool

func init() {
	statusCmd.Flags().BoolVar(&showConfigDetails, "config", false, "Show configuration details")
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

	// Show project status tree
	if err := renderStatusTree(ctxManager); err != nil {
		return fmt.Errorf("failed to render status tree: %w", err)
	}

	fmt.Println()

	// Show configuration status
	if showConfigDetails {
		fmt.Println("Configuration:")
		showConfigStatus()
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
