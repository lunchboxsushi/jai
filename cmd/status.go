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

	// Get current context
	currentCtx := ctxManager.Get()

	// Show current context
	fmt.Println("Current Context:")
	if currentCtx.EpicKey == "" && currentCtx.TaskKey == "" {
		fmt.Println("  No context set")
	} else {
		parser := markdown.NewParser(dataDir)
		if currentCtx.EpicKey != "" {
			epicTitle := ""
			// Try to resolve the actual file and extract the title
			actualFilePath, err := findEpicFileByKey(dataDir, currentCtx.EpicKey)
			if err == nil {
				mdFile, err := parser.ParseFile(actualFilePath)
				if err == nil {
					for _, ticket := range mdFile.Tickets {
						if ticket.Key == currentCtx.EpicKey {
							epicTitle = ticket.Title
							break
						}
					}
				}
			}
			if epicTitle != "" {
				fmt.Printf("  Epic: %s [%s]\n", parser.RemoveJiraKey(epicTitle), currentCtx.EpicKey)
			} else {
				fmt.Printf("  Epic: [%s]\n", currentCtx.EpicKey)
			}
		}
		if currentCtx.TaskKey != "" {
			taskTitle := ""
			// Try to resolve the actual file and extract the title
			actualFilePath, err := findTaskFileByKey(dataDir, currentCtx.TaskKey)
			if err == nil {
				mdFile, err := parser.ParseFile(actualFilePath)
				if err == nil {
					for _, ticket := range mdFile.Tickets {
						if ticket.Key == currentCtx.TaskKey {
							taskTitle = ticket.Title
							break
						}
					}
				}
			}
			if taskTitle != "" {
				fmt.Printf("  Task: %s [%s]\n", parser.RemoveJiraKey(taskTitle), currentCtx.TaskKey)
			} else {
				fmt.Printf("  Task: [%s]\n", currentCtx.TaskKey)
			}
		} else {
			fmt.Println("  No Tasks")
		}
		fmt.Printf("  Last Updated: %s\n", currentCtx.Updated.Format(time.RFC3339))
	}

	fmt.Println()

	// Show configuration status
	if showConfigDetails {
		fmt.Println("Configuration:")
		showConfigStatus()
	}

	return nil
}

// findEpicFileByKey searches for the actual epic file that contains the given key
func findEpicFileByKey(dataDir, epicKey string) (string, error) {
	ticketsDir := filepath.Join(dataDir, "tickets")
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		return "", err
	}

	parser := markdown.NewParser(dataDir)
	for _, file := range files {
		if file.IsDir() || !isMarkdownFile(file.Name()) {
			continue
		}
		filePath := filepath.Join(ticketsDir, file.Name())
		mdFile, err := parser.ParseFile(filePath)
		if err != nil {
			continue
		}
		for _, ticket := range mdFile.Tickets {
			if ticket.Key == epicKey {
				return filePath, nil
			}
		}
	}

	return "", fmt.Errorf("epic file not found for key: %s", epicKey)
}

// findTaskFileByKey searches for the actual task file that contains the given key
func findTaskFileByKey(dataDir, taskKey string) (string, error) {
	ticketsDir := filepath.Join(dataDir, "tickets")
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		return "", err
	}

	parser := markdown.NewParser(dataDir)
	for _, file := range files {
		if file.IsDir() || !isMarkdownFile(file.Name()) {
			continue
		}
		filePath := filepath.Join(ticketsDir, file.Name())
		mdFile, err := parser.ParseFile(filePath)
		if err != nil {
			continue
		}
		for _, ticket := range mdFile.Tickets {
			if ticket.Key == taskKey {
				return filePath, nil
			}
		}
	}

	return "", fmt.Errorf("task file not found for key: %s", taskKey)
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
