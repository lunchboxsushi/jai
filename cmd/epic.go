package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var epicCmd = &cobra.Command{
	Use:   "epic [title|key]",
	Short: "Set or switch current epic context",
	Long: `Set or switch the current epic context. This will be used as the parent for new tasks.

Examples:
  jai epic "SRE-5912"           # Set epic by key
  jai epic "Observability Refactor"  # Set epic by title (fuzzy match)
  jai epic                      # Show current epic context`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEpic,
}

func init() {
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

	// If no args provided, show current context
	if len(args) == 0 {
		fmt.Println(ctxManager.String())
		return nil
	}

	epicArg := args[0]

	// Check if it's a Jira key (format: PROJECT-123)
	if isJiraKey(epicArg) {
		// Set epic by key
		if err := ctxManager.SetEpic(epicArg, ""); err != nil {
			return fmt.Errorf("failed to set epic context: %w", err)
		}
		fmt.Printf("Set epic context to: %s\n", epicArg)
		return nil
	}

	// Try to find epic by title (fuzzy match)
	parser := markdown.NewParser(dataDir)
	ticketsDir := filepath.Join(dataDir, "tickets")

	// Search through markdown files for matching epic
	foundEpic, err := findEpicByTitle(parser, ticketsDir, epicArg)
	if err != nil {
		return fmt.Errorf("failed to search for epic: %w", err)
	}

	if foundEpic != nil {
		if err := ctxManager.SetEpic(foundEpic.Key, foundEpic.ID); err != nil {
			return fmt.Errorf("failed to set epic context: %w", err)
		}
		fmt.Printf("Set epic context to: %s [%s]\n", foundEpic.Title, foundEpic.Key)
		return nil
	}

	// If not found, create a new epic file
	epicKey := generateEpicKey(epicArg)
	if err := ctxManager.SetEpic(epicKey, ""); err != nil {
		return fmt.Errorf("failed to set epic context: %w", err)
	}

	// Create the epic file
	epicFilePath := parser.GetEpicFilePath(epicKey)
	if err := parser.EnsureFileExists(epicFilePath); err != nil {
		return fmt.Errorf("failed to create epic file: %w", err)
	}

	fmt.Printf("Created new epic context: %s [%s]\n", epicArg, epicKey)
	return nil
}

// isJiraKey checks if a string looks like a Jira key
func isJiraKey(s string) bool {
	// Simple regex for PROJECT-123 format
	re := regexp.MustCompile(`^[A-Z]+-\d+$`)
	return re.MatchString(s)
}

// findEpicByTitle searches for an epic by title
func findEpicByTitle(parser *markdown.Parser, ticketsDir string, title string) (*types.Ticket, error) {
	// Read all markdown files in the tickets directory
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory doesn't exist, no epics found
		}
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() || !isMarkdownFile(file.Name()) {
			continue
		}

		filePath := filepath.Join(ticketsDir, file.Name())
		mdFile, err := parser.ParseFile(filePath)
		if err != nil {
			continue // Skip files that can't be parsed
		}

		// Look for epics in this file
		for _, ticket := range mdFile.Tickets {
			if ticket.Type == types.TicketTypeEpic {
				// Simple fuzzy matching - check if title contains the search term
				if strings.Contains(strings.ToLower(ticket.Title), strings.ToLower(title)) {
					return &ticket, nil
				}
			}
		}
	}

	return nil, nil
}

// generateEpicKey generates a Jira-style key from a title
func generateEpicKey(title string) string {
	// Extract project prefix from config or use default
	project := viper.GetString("jira.project")
	if project == "" {
		project = "PROJ"
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

// isMarkdownFile checks if a file is a markdown file
func isMarkdownFile(filename string) bool {
	return strings.HasSuffix(filename, ".md")
}
