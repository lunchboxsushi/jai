package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lunchboxsushi/jai/internal/jira"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var importCmd = &cobra.Command{
	Use:   "import [ticket-id]",
	Short: "Import a Jira ticket and save it as a markdown file",
	Long: `Import a Jira ticket by its ID (e.g., "SRE-5573") and save it as a markdown file.

This command will:
- Fetch the ticket from Jira using the configured API credentials
- Parse the ticket content and metadata
- Save it to ~/.local/share/jai/tickets/ directory
- Recursively import parent tickets for subtasks and tasks (if they have epics)

Examples:
  jai import SRE-5573        # Import a specific ticket
  jai import "SRE-5573"      # Import with quotes (equivalent)`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	ticketID := strings.TrimSpace(args[0])
	if ticketID == "" {
		return fmt.Errorf("ticket ID cannot be empty")
	}

	// Get data directory from config
	dataDir := viper.GetString("general.data_dir")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share", "jai")
	}

	// Ensure tickets directory exists
	ticketsDir := filepath.Join(dataDir, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tickets directory: %w", err)
	}

	// Create Jira client
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
		return fmt.Errorf("Jira configuration incomplete. Please check:\n- jira.url in config\n- jira.username in config\n- JAI_JIRA_TOKEN environment variable")
	}

	jiraClient, err := jira.NewClient(jiraConfig)
	if err != nil {
		return fmt.Errorf("failed to create Jira client: %w", err)
	}

	// Initialize markdown parser
	parser := markdown.NewParser(dataDir)

	fmt.Printf("Importing ticket: %s\n", ticketID)

	// Import the main ticket and any parent tickets
	importedTickets, err := importTicketRecursively(jiraClient, parser, ticketID, make(map[string]bool))
	if err != nil {
		return fmt.Errorf("failed to import ticket: %w", err)
	}

	fmt.Printf("Successfully imported %d ticket(s):\n", len(importedTickets))
	for _, ticket := range importedTickets {
		fmt.Printf("  - %s: %s (%s)\n", ticket.Key, ticket.Title, ticket.Type)
	}

	return nil
}

// importTicketRecursively imports a ticket and its parent/child tickets recursively
func importTicketRecursively(jiraClient *jira.Client, parser *markdown.Parser, ticketID string, imported map[string]bool) ([]*types.Ticket, error) {
	var allTickets []*types.Ticket

	// Skip if already imported
	if imported[ticketID] {
		return allTickets, nil
	}

	fmt.Printf("Fetching ticket: %s\n", ticketID)

	// Fetch the ticket from Jira
	ticket, err := jiraClient.GetTicket(ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticket %s: %w", ticketID, err)
	}

	// Mark as imported
	imported[ticketID] = true

	// Import parent tickets first based on ticket type
	switch ticket.Type {
	case types.TicketTypeSubtask:
		// For subtasks, import the parent task if it exists
		if ticket.ParentKey != "" {
			fmt.Printf("  → Importing parent task: %s\n", ticket.ParentKey)
			parentTickets, err := importTicketRecursively(jiraClient, parser, ticket.ParentKey, imported)
			if err != nil {
				fmt.Printf("  Warning: Failed to import parent task %s: %v\n", ticket.ParentKey, err)
			} else {
				allTickets = append(allTickets, parentTickets...)
			}
		}
	case types.TicketTypeTask, types.TicketTypeSpike:
		// For tasks and spikes, import the epic if it exists
		if ticket.EpicKey != "" {
			fmt.Printf("  → Importing parent epic: %s\n", ticket.EpicKey)
			epicTickets, err := importTicketRecursively(jiraClient, parser, ticket.EpicKey, imported)
			if err != nil {
				fmt.Printf("  Warning: Failed to import parent epic %s: %v\n", ticket.EpicKey, err)
			} else {
				allTickets = append(allTickets, epicTickets...)
			}
		}
	case types.TicketTypeEpic:
		// Epics don't have parents, so nothing to import
	}

	// Import child tickets based on ticket type
	switch ticket.Type {
	case types.TicketTypeEpic:
		// For epics, import all child tasks and spikes
		childTasks, err := findChildTickets(jiraClient, ticket.Key, "Task")
		if err != nil {
			fmt.Printf("  Warning: Failed to find child tasks for epic %s: %v\n", ticket.Key, err)
		} else {
			for _, childTask := range childTasks {
				if !imported[childTask.Key] {
					fmt.Printf("  → Importing child task: %s\n", childTask.Key)
					taskTickets, err := importTicketRecursively(jiraClient, parser, childTask.Key, imported)
					if err != nil {
						fmt.Printf("  Warning: Failed to import child task %s: %v\n", childTask.Key, err)
					} else {
						allTickets = append(allTickets, taskTickets...)
					}
				}
			}
		}

		// Also import child spikes
		childSpikes, err := findChildTickets(jiraClient, ticket.Key, "Spike")
		if err != nil {
			fmt.Printf("  Warning: Failed to find child spikes for epic %s: %v\n", ticket.Key, err)
		} else {
			for _, childSpike := range childSpikes {
				if !imported[childSpike.Key] {
					fmt.Printf("  → Importing child spike: %s\n", childSpike.Key)
					spikeTickets, err := importTicketRecursively(jiraClient, parser, childSpike.Key, imported)
					if err != nil {
						fmt.Printf("  Warning: Failed to import child spike %s: %v\n", childSpike.Key, err)
					} else {
						allTickets = append(allTickets, spikeTickets...)
					}
				}
			}
		}
	case types.TicketTypeTask, types.TicketTypeSpike:
		// For tasks and spikes, import all child subtasks
		childSubtasks, err := findChildTickets(jiraClient, ticket.Key, "Sub-task")
		if err != nil {
			fmt.Printf("  Warning: Failed to find child subtasks for %s %s: %v\n", ticket.Type, ticket.Key, err)
		} else {
			for _, childSubtask := range childSubtasks {
				if !imported[childSubtask.Key] {
					fmt.Printf("  → Importing child subtask: %s\n", childSubtask.Key)
					subtaskTickets, err := importTicketRecursively(jiraClient, parser, childSubtask.Key, imported)
					if err != nil {
						fmt.Printf("  Warning: Failed to import child subtask %s: %v\n", childSubtask.Key, err)
					} else {
						allTickets = append(allTickets, subtaskTickets...)
					}
				}
			}
		}
	case types.TicketTypeSubtask:
		// Subtasks don't have children
	}

	// Save the ticket as a markdown file
	if err := saveTicketToMarkdown(parser, ticket); err != nil {
		return nil, fmt.Errorf("failed to save ticket %s: %w", ticketID, err)
	}

	allTickets = append(allTickets, ticket)
	return allTickets, nil
}

// findChildTickets finds child tickets of a given parent using JQL
func findChildTickets(jiraClient *jira.Client, parentKey string, childType string) ([]*types.Ticket, error) {
	var jql string

	switch childType {
	case "Task":
		// Find tasks that belong to this epic
		// Note: This assumes the epic link field is properly configured
		jql = fmt.Sprintf("\"Epic Link\" = %s AND type = Task", parentKey)
	case "Spike":
		// Find spikes that belong to this epic
		jql = fmt.Sprintf("\"Epic Link\" = %s AND type = Spike", parentKey)
	case "Sub-task":
		// Find subtasks that have this task or spike as parent
		jql = fmt.Sprintf("parent = %s AND type = Sub-task", parentKey)
	default:
		return nil, fmt.Errorf("unsupported child type: %s", childType)
	}

	fmt.Printf("  → Searching for %s children with JQL: %s\n", childType, jql)

	tickets, err := jiraClient.SearchTickets(jql)
	if err != nil {
		return nil, fmt.Errorf("failed to search for child tickets: %w", err)
	}

	fmt.Printf("  → Found %d %s children\n", len(tickets), childType)
	return tickets, nil
}

// saveTicketToMarkdown saves a ticket to a markdown file
func saveTicketToMarkdown(parser *markdown.Parser, ticket *types.Ticket) error {
	// Create filename based on ticket key and title
	safeTitle := sanitizeFilename(ticket.Title)
	filename := fmt.Sprintf("%s-%s.md", ticket.Key, safeTitle)
	filePath := filepath.Join(parser.GetTicketsDir(), filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		fmt.Printf("  → File already exists: %s (skipping)\n", filename)
		return nil
	}

	// Generate markdown content for the ticket
	content := generateImportedTicketMarkdown(ticket)

	// Write to file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	fmt.Printf("  → Saved: %s\n", filename)
	return nil
}

// generateImportedTicketMarkdown generates markdown content for an imported ticket
func generateImportedTicketMarkdown(ticket *types.Ticket) string {
	var lines []string

	// Add parent references at the top
	switch ticket.Type {
	case types.TicketTypeSubtask:
		if ticket.ParentKey != "" {
			lines = append(lines, fmt.Sprintf("**Parent Task:** [%s](%s-%s.md)", ticket.ParentKey, ticket.ParentKey, sanitizeFilename("task")))
			lines = append(lines, "")
		}
		if ticket.EpicKey != "" {
			lines = append(lines, fmt.Sprintf("**Epic:** [%s](%s-%s.md)", ticket.EpicKey, ticket.EpicKey, sanitizeFilename("epic")))
			lines = append(lines, "")
		}
	case types.TicketTypeTask, types.TicketTypeSpike:
		if ticket.EpicKey != "" {
			lines = append(lines, fmt.Sprintf("**Epic:** [%s](%s-%s.md)", ticket.EpicKey, ticket.EpicKey, sanitizeFilename("epic")))
			lines = append(lines, "")
		}
	}

	// Add ticket header based on type
	var header string
	switch ticket.Type {
	case types.TicketTypeEpic:
		header = fmt.Sprintf("# epic: %s [%s]", ticket.Title, ticket.Key)
	case types.TicketTypeTask:
		header = fmt.Sprintf("## task: %s [%s]", ticket.Title, ticket.Key)
	case types.TicketTypeSubtask:
		header = fmt.Sprintf("### subtask: %s [%s]", ticket.Title, ticket.Key)
	case types.TicketTypeSpike:
		header = fmt.Sprintf("#### spike: %s [%s]", ticket.Title, ticket.Key)
	}
	lines = append(lines, header)
	lines = append(lines, "")

	// Add description if available
	if ticket.Description != "" {
		lines = append(lines, ticket.Description)
		lines = append(lines, "")
	}

	// Add metadata section
	lines = append(lines, "---")
	lines = append(lines, "*Metadata:*")
	lines = append(lines, fmt.Sprintf("- Key: %s", ticket.Key))
	lines = append(lines, fmt.Sprintf("- Status: %s", ticket.Status))

	if ticket.Priority != "" {
		lines = append(lines, fmt.Sprintf("- Priority: %s", ticket.Priority))
	}

	if len(ticket.Labels) > 0 {
		lines = append(lines, fmt.Sprintf("- Labels: %s", strings.Join(ticket.Labels, ", ")))
	}

	// Add appropriate parent key metadata based on ticket type
	switch ticket.Type {
	case types.TicketTypeTask, types.TicketTypeSpike:
		if ticket.EpicKey != "" {
			lines = append(lines, fmt.Sprintf("- ParentKey: %s", ticket.EpicKey))
		}
	case types.TicketTypeSubtask:
		if ticket.ParentKey != "" {
			lines = append(lines, fmt.Sprintf("- TaskKey: %s", ticket.ParentKey))
		}
	}

	if ticket.Assignee != "" {
		lines = append(lines, fmt.Sprintf("- Assignee: %s", ticket.Assignee))
	}

	lines = append(lines, fmt.Sprintf("- Created: %s", ticket.Created.Format("2006-01-02 15:04:05")))
	lines = append(lines, fmt.Sprintf("- Updated: %s", ticket.Updated.Format("2006-01-02 15:04:05")))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// sanitizeFilename removes or replaces characters that are not safe for filenames
func sanitizeFilename(name string) string {
	// Replace spaces and unsafe characters with hyphens
	safe := strings.ReplaceAll(name, " ", "-")
	safe = strings.ReplaceAll(safe, "/", "-")
	safe = strings.ReplaceAll(safe, "\\", "-")
	safe = strings.ReplaceAll(safe, ":", "-")
	safe = strings.ReplaceAll(safe, "*", "-")
	safe = strings.ReplaceAll(safe, "?", "-")
	safe = strings.ReplaceAll(safe, "\"", "-")
	safe = strings.ReplaceAll(safe, "<", "-")
	safe = strings.ReplaceAll(safe, ">", "-")
	safe = strings.ReplaceAll(safe, "|", "-")

	// Remove any double dashes and trim
	safe = strings.ReplaceAll(safe, "--", "-")
	safe = strings.Trim(safe, "-")

	// Limit length to avoid filesystem issues
	if len(safe) > 50 {
		safe = safe[:50]
		safe = strings.Trim(safe, "-")
	}

	return safe
}
