package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

	// If no args provided, show interactive epic/task selection
	if len(args) == 0 {
		return interactiveFocus(ctxManager, dataDir)
	}

	query := args[0]

	// Check if it's a Jira key
	if isJiraKey(query) {
		return focusByKey(ctxManager, query)
	}

	// Try fuzzy matching
	return focusByFuzzyMatch(ctxManager, dataDir, query)
}

// interactiveFocus provides a hierarchical selection: epics -> tasks -> subtasks
func interactiveFocus(ctxManager *context.Manager, dataDir string) error {
	parser := markdown.NewParser(dataDir)
	ticketsDir := filepath.Join(dataDir, "tickets")

	// 1. List all epics
	epics, err := listEpics(parser, ticketsDir)
	if err != nil {
		return fmt.Errorf("failed to list epics: %w", err)
	}
	if len(epics) == 0 {
		return fmt.Errorf("no epics found")
	}

	fmt.Println("Select an epic:")
	for i, epic := range epics {
		fmt.Printf("%d. %s [%s]\n", i+1, parser.RemoveJiraKey(epic.Title), epic.Key)
	}
	fmt.Print("Enter number (or blank to cancel): ")
	selectedEpicIdx := readNumber(len(epics))
	if selectedEpicIdx == -1 {
		fmt.Println("Cancelled.")
		return nil
	}
	epic := epics[selectedEpicIdx]
	if err := ctxManager.SetEpic(epic.Key, epic.ID); err != nil {
		return fmt.Errorf("failed to set epic context: %w", err)
	}
	fmt.Printf("Focused on epic: %s [%s]\n", parser.RemoveJiraKey(epic.Title), epic.Key)

	// 2. List tasks and subtasks under the selected epic
	tasks, err := listTasksForEpic(parser, ticketsDir, epic.Key)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}
	subtasks, err := listSubtasksForEpic(parser, ticketsDir, epic.Key)
	if err != nil {
		return fmt.Errorf("failed to list subtasks: %w", err)
	}
	if len(tasks) == 0 && len(subtasks) == 0 {
		fmt.Println("No tasks or subtasks found under this epic.")
		return nil
	}

	fmt.Println("Select a task or subtask:")
	combined := make([]types.Ticket, 0, len(tasks)+len(subtasks))
	for _, task := range tasks {
		combined = append(combined, task)
	}
	for _, subtask := range subtasks {
		combined = append(combined, subtask)
	}
	for i, ticket := range combined {
		typeLabel := "Task"
		if ticket.Type == types.TicketTypeSubtask {
			typeLabel = "Subtask"
		}
		fmt.Printf("%d. %s [%s] (%s)\n", i+1, parser.RemoveJiraKey(ticket.Title), ticket.Key, typeLabel)
	}
	fmt.Print("Enter number (or blank to stay on epic): ")
	selectedIdx := readNumber(len(combined))
	if selectedIdx == -1 {
		fmt.Println("Staying focused on epic.")
		return nil
	}
	ticket := combined[selectedIdx]

	// Set context based on ticket type
	switch ticket.Type {
	case types.TicketTypeTask:
		if err := ctxManager.SetTask(ticket.Key, ticket.ID); err != nil {
			return fmt.Errorf("failed to set task context: %w", err)
		}
		fmt.Printf("Focused on task: %s [%s]\n", parser.RemoveJiraKey(ticket.Title), ticket.Key)

		// Optionally show subtasks under this task
		subtasks, err := listSubtasksForTask(parser, ticketsDir, ticket.Key)
		if err != nil {
			fmt.Printf("Warning: Failed to list subtasks: %v\n", err)
		} else if len(subtasks) > 0 {
			fmt.Printf("Found %d subtasks under this task.\n", len(subtasks))
		}

	case types.TicketTypeSubtask:
		// For subtasks, set both epic and task context
		if ticket.EpicKey != "" && ticket.ParentKey != "" {
			if err := ctxManager.SetEpicAndTask(ticket.EpicKey, "", ticket.ParentKey, ""); err != nil {
				return fmt.Errorf("failed to set epic and task context: %w", err)
			}
		} else if ticket.EpicKey != "" {
			if err := ctxManager.SetEpic(ticket.EpicKey, ""); err != nil {
				return fmt.Errorf("failed to set epic context: %w", err)
			}
		} else if ticket.ParentKey != "" {
			if err := ctxManager.SetTask(ticket.ParentKey, ""); err != nil {
				return fmt.Errorf("failed to set task context: %w", err)
			}
		}
		fmt.Printf("Focused on subtask: %s [%s]\n", parser.RemoveJiraKey(ticket.Title), ticket.Key)
	}
	return nil
}

// listEpics returns all epics from all markdown files
func listEpics(parser *markdown.Parser, ticketsDir string) ([]types.Ticket, error) {
	var epics []types.Ticket
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return epics, nil
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
			continue
		}
		for _, ticket := range mdFile.Tickets {
			if ticket.Type == types.TicketTypeEpic {
				epics = append(epics, ticket)
			}
		}
	}
	return epics, nil
}

// listTasksForEpic returns all tasks for a given epic key
func listTasksForEpic(parser *markdown.Parser, ticketsDir string, epicKey string) ([]types.Ticket, error) {
	var tasks []types.Ticket
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return tasks, nil
		}
		return nil, err
	}
	epicKeyNorm := strings.TrimSpace(strings.ToUpper(epicKey))
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
			if ticket.Type == types.TicketTypeTask {
				// Check both EpicKey and ParentKey (for backward compatibility)
				ticketEpicKey := strings.TrimSpace(strings.ToUpper(ticket.EpicKey))
				if ticketEpicKey == epicKeyNorm {
					tasks = append(tasks, ticket)
				}
			}
		}
	}
	if len(tasks) == 0 {
		fmt.Printf("[debug] No tasks found for epic key '%s'.\n", epicKey)
		fmt.Println("[debug] Check that your task files have the correct ParentKey field set and match the selected epic.")
	}
	return tasks, nil
}

// listSubtasksForEpic returns all subtasks for a given epic key
func listSubtasksForEpic(parser *markdown.Parser, ticketsDir string, epicKey string) ([]types.Ticket, error) {
	var subtasks []types.Ticket
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return subtasks, nil
		}
		return nil, err
	}
	epicKeyNorm := strings.TrimSpace(strings.ToUpper(epicKey))
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
			if ticket.Type == types.TicketTypeSubtask {
				// Check EpicKey for subtasks
				parentEpic := strings.TrimSpace(strings.ToUpper(ticket.EpicKey))
				if parentEpic == epicKeyNorm {
					subtasks = append(subtasks, ticket)
				}
			}
		}
	}
	return subtasks, nil
}

// listSubtasksForTask returns all subtasks for a given task key
func listSubtasksForTask(parser *markdown.Parser, ticketsDir string, taskKey string) ([]types.Ticket, error) {
	var subtasks []types.Ticket
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return subtasks, nil
		}
		return nil, err
	}
	taskKeyNorm := strings.TrimSpace(strings.ToUpper(taskKey))
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
			if ticket.Type == types.TicketTypeSubtask {
				// Check TaskKey (ParentKey field) for subtasks
				parentTask := strings.TrimSpace(strings.ToUpper(ticket.ParentKey))
				if parentTask == taskKeyNorm {
					subtasks = append(subtasks, ticket)
				}
			}
		}
	}
	return subtasks, nil
}

// readNumber reads a number from stdin, returns -1 if blank/cancel
func readNumber(max int) int {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return -1
	}
	var n int
	_, err := fmt.Sscanf(input, "%d", &n)
	if err != nil || n < 1 || n > max {
		fmt.Println("Invalid selection.")
		return -1
	}
	return n - 1
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
		return setTicketContext(ctxManager, parser, ticket)
	}

	// Multiple matches, show selection
	fmt.Printf("Multiple matches found for '%s':\n", query)
	for i, ticket := range matches {
		fmt.Printf("%d. %s [%s] (%s)\n", i+1, parser.RemoveJiraKey(ticket.Title), ticket.Key, ticket.Type)
	}

	// For now, just use the first match
	// In a full implementation, you'd want interactive selection
	ticket := matches[0]
	fmt.Printf("Using first match: %s [%s]\n", parser.RemoveJiraKey(ticket.Title), ticket.Key)
	return setTicketContext(ctxManager, parser, ticket)
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
func setTicketContext(ctxManager *context.Manager, parser *markdown.Parser, ticket types.Ticket) error {
	switch ticket.Type {
	case types.TicketTypeEpic:
		if err := ctxManager.SetEpic(ticket.Key, ticket.ID); err != nil {
			return fmt.Errorf("failed to set epic context: %w", err)
		}
		fmt.Printf("Focused on epic: %s [%s]\n", parser.RemoveJiraKey(ticket.Title), ticket.Key)
	case types.TicketTypeTask:
		// For tasks, set both epic and task context if epic is available
		if ticket.EpicKey != "" {
			if err := ctxManager.SetEpicAndTask(ticket.EpicKey, "", ticket.Key, ticket.ID); err != nil {
				return fmt.Errorf("failed to set epic and task context: %w", err)
			}
		} else {
			if err := ctxManager.SetTask(ticket.Key, ticket.ID); err != nil {
				return fmt.Errorf("failed to set task context: %w", err)
			}
		}
		fmt.Printf("Focused on task: %s [%s]\n", parser.RemoveJiraKey(ticket.Title), ticket.Key)
	case types.TicketTypeSubtask:
		// For subtasks, set epic, task, and subtask context
		if ticket.EpicKey != "" && ticket.ParentKey != "" {
			if err := ctxManager.SetEpicAndTask(ticket.EpicKey, "", ticket.ParentKey, ""); err != nil {
				return fmt.Errorf("failed to set epic and task context: %w", err)
			}
		} else if ticket.EpicKey != "" {
			if err := ctxManager.SetEpic(ticket.EpicKey, ""); err != nil {
				return fmt.Errorf("failed to set epic context: %w", err)
			}
		} else if ticket.ParentKey != "" {
			if err := ctxManager.SetTask(ticket.ParentKey, ""); err != nil {
				return fmt.Errorf("failed to set task context: %w", err)
			}
		}
		fmt.Printf("Focused on subtask: %s [%s]\n", parser.RemoveJiraKey(ticket.Title), ticket.Key)
	}

	return nil
}

// isJiraKey checks if a string looks like a Jira key
func isJiraKey(s string) bool {
	// Simple regex for PROJECT-123 format
	re := regexp.MustCompile(`^[A-Z]+-\d+$`)
	return re.MatchString(s)
}

// isMarkdownFile checks if a file is a markdown file
func isMarkdownFile(filename string) bool {
	return strings.HasSuffix(filename, ".md")
}
