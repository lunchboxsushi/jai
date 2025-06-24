package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var newCmd = &cobra.Command{
	Use:   "new [content]",
	Short: "Quickly append a task or sub-task to current context",
	Long: `Quickly append a new task or sub-task to the current context without opening an editor.

Examples:
  jai new "fix login bug"              # Add task under current epic
  jai new "add unit tests"             # Add subtask under current task
  jai new                              # Interactive mode`,
	RunE: runNew,
}

func init() {
	newCmd.Flags().BoolVar(&noEnrich, "no-enrich", false, "Skip AI enrichment")
	newCmd.Flags().BoolVar(&noCreate, "no-create", false, "Skip Jira ticket creation")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
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

	// Determine content
	var content string
	if len(args) > 0 {
		content = strings.Join(args, " ")
	} else {
		// Interactive mode
		fmt.Print("Enter task description: ")
		fmt.Scanln(&content)
	}

	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("no content provided")
	}

	// Determine ticket type based on context
	var ticketType types.TicketType
	var epicKey, parentKey string

	if currentCtx.EpicKey == "" {
		return fmt.Errorf("no epic context set. Use 'jai epic <key|title>' first")
	}

	epicKey = currentCtx.EpicKey

	if currentCtx.TaskKey != "" {
		// We have a task context, create a subtask
		ticketType = types.TicketTypeSubtask
		parentKey = currentCtx.TaskKey
	} else {
		// We only have epic context, create a task
		ticketType = types.TicketTypeTask
	}

	// Create ticket
	ticket := &types.Ticket{
		Type:       ticketType,
		Title:      content,
		RawContent: content,
		EpicKey:    epicKey,
		ParentKey:  parentKey,
		Created:    time.Now(),
		Updated:    time.Now(),
	}

	// Enrich with AI if enabled
	if !noEnrich {
		fmt.Println("Enriching with AI...")
		enriched, err := enrichTask(ticket, currentCtx)
		if err != nil {
			fmt.Printf("Warning: AI enrichment failed: %v\n", err)
		} else {
			ticket.Enriched = enriched.Description
			ticket.Title = enriched.Title
			ticket.Description = enriched.Description
			if len(enriched.Labels) > 0 {
				ticket.Labels = enriched.Labels
			}
			if enriched.Priority != "" {
				ticket.Priority = enriched.Priority
			}
		}
	}

	// Add to epic file
	parser := markdown.NewParser(dataDir)
	epicFilePath := parser.GetEpicFilePath(epicKey)

	// Ensure epic file exists
	if err := parser.EnsureFileExists(epicFilePath); err != nil {
		return fmt.Errorf("failed to create epic file: %w", err)
	}

	// Add ticket to file
	if err := addTicketToEpicFile(parser, epicFilePath, ticket); err != nil {
		return fmt.Errorf("failed to add ticket to epic file: %w", err)
	}

	// Show what was created
	ticketTypeStr := "task"
	if ticketType == types.TicketTypeSubtask {
		ticketTypeStr = "subtask"
	}
	fmt.Printf("%s added to epic %s\n", strings.Title(ticketTypeStr), epicKey)

	// Create Jira ticket if enabled
	if !noCreate {
		fmt.Println("Creating Jira ticket...")
		if err := createJiraTicket(ticket); err != nil {
			fmt.Printf("Warning: Failed to create Jira ticket: %v\n", err)
		} else {
			fmt.Printf("Jira ticket created: %s\n", ticket.Key)
		}
	}

	return nil
}

// addTicketToEpicFile adds a ticket to the epic markdown file
func addTicketToEpicFile(parser *markdown.Parser, epicFilePath string, ticket *types.Ticket) error {
	// Parse existing file
	mdFile, err := parser.ParseFile(epicFilePath)
	if err != nil {
		// File might not exist or be empty, start fresh
		mdFile = &types.MarkdownFile{
			Path:    epicFilePath,
			Tickets: []types.Ticket{},
		}
	}

	// Add the new ticket
	mdFile.Tickets = append(mdFile.Tickets, *ticket)

	// Write back to file
	return parser.WriteFile(epicFilePath, mdFile.Tickets)
}
