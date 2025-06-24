package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var focusCmd = &cobra.Command{
	Use:   "focus <query>",
	Short: "Set current context by fuzzy-matching epic/task title",
	Long: `Set current context by fuzzy-matching epic or task titles.

Examples:
  jai focus "observability"     # Focus on epic/task containing "observability"
  jai focus "SRE-1234"          # Focus on specific ticket by key
  jai focus                     # Show current focus`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFocus,
}

func init() {
	rootCmd.AddCommand(focusCmd)
}

func runFocus(cmd *cobra.Command, args []string) error {
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

	query := args[0]

	// Check if it's a Jira key
	if isJiraKey(query) {
		return focusByKey(ctxManager, query)
	}

	// Try fuzzy matching
	return focusByFuzzyMatch(ctxManager, dataDir, query)
}

// focusByKey focuses on a specific ticket by key
func focusByKey(ctxManager *context.Manager, key string) error {
	// For now, we'll just set it as the task context
	// In a full implementation, you'd want to verify the key exists in Jira
	if err := ctxManager.SetTask(key, ""); err != nil {
		return fmt.Errorf("failed to set task context: %w", err)
	}

	fmt.Printf("Focused on task: %s\n", key)
	return nil
}

// focusByFuzzyMatch focuses on a ticket by fuzzy matching the title
func focusByFuzzyMatch(ctxManager *context.Manager, dataDir string, query string) error {
	parser := markdown.NewParser(dataDir)
	ticketsDir := filepath.Join(dataDir, "tickets")

	// Search for matching tickets
	matches, err := searchTickets(parser, ticketsDir, query)
	if err != nil {
		return fmt.Errorf("failed to search tickets: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("no tickets found matching '%s'", query)
	}

	if len(matches) == 1 {
		// Single match, set it as context
		ticket := matches[0]
		return setTicketContext(ctxManager, ticket)
	}

	// Multiple matches, show selection
	fmt.Printf("Multiple matches found for '%s':\n", query)
	for i, ticket := range matches {
		fmt.Printf("%d. %s [%s] (%s)\n", i+1, ticket.Title, ticket.Key, ticket.Type)
	}

	// For now, just use the first match
	// In a full implementation, you'd want interactive selection
	ticket := matches[0]
	fmt.Printf("Using first match: %s [%s]\n", ticket.Title, ticket.Key)
	return setTicketContext(ctxManager, ticket)
}

// searchTickets searches for tickets matching a query
func searchTickets(parser *markdown.Parser, ticketsDir string, query string) ([]types.Ticket, error) {
	var matches []types.Ticket

	// Read all markdown files in the tickets directory
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return matches, nil // Directory doesn't exist, no tickets found
		}
		return nil, err
	}

	queryLower := strings.ToLower(query)

	for _, file := range files {
		if file.IsDir() || !isMarkdownFile(file.Name()) {
			continue
		}

		filePath := filepath.Join(ticketsDir, file.Name())
		mdFile, err := parser.ParseFile(filePath)
		if err != nil {
			continue // Skip files that can't be parsed
		}

		// Look for matching tickets in this file
		for _, ticket := range mdFile.Tickets {
			titleLower := strings.ToLower(ticket.Title)
			if strings.Contains(titleLower, queryLower) {
				matches = append(matches, ticket)
			}
		}
	}

	return matches, nil
}

// setTicketContext sets the appropriate context based on ticket type
func setTicketContext(ctxManager *context.Manager, ticket types.Ticket) error {
	switch ticket.Type {
	case types.TicketTypeEpic:
		if err := ctxManager.SetEpic(ticket.Key, ticket.ID); err != nil {
			return fmt.Errorf("failed to set epic context: %w", err)
		}
		fmt.Printf("Focused on epic: %s [%s]\n", ticket.Title, ticket.Key)
	case types.TicketTypeTask:
		if err := ctxManager.SetTask(ticket.Key, ticket.ID); err != nil {
			return fmt.Errorf("failed to set task context: %w", err)
		}
		fmt.Printf("Focused on task: %s [%s]\n", ticket.Title, ticket.Key)
	case types.TicketTypeSubtask:
		if err := ctxManager.SetTask(ticket.Key, ticket.ID); err != nil {
			return fmt.Errorf("failed to set subtask context: %w", err)
		}
		fmt.Printf("Focused on subtask: %s [%s]\n", ticket.Title, ticket.Key)
	}

	return nil
}
