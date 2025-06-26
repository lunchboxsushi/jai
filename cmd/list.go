package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	treepkg "github.com/charmbracelet/lipgloss/tree"
	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List all tickets in tree structure",
	Long: `List all tickets in a hierarchical tree structure. Shows epics with their tasks and subtasks.
Orphan tasks (tasks without epics) are shown at the end.

Examples:
  jai list              # Show all tickets in tree structure
  jai list epic         # Show only epics
  jai list task         # Show only tasks
  jai list subtask      # Show only subtasks
  jai list orphan       # Show only orphan tasks`,
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	// Get data directory from config
	dataDir := viper.GetString("general.data_dir")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share", "jai")
	}

	// Initialize context manager to show current focus
	ctxManager := context.NewManager(dataDir)
	if err := ctxManager.Load(); err != nil {
		return fmt.Errorf("failed to load context: %w", err)
	}

	parser := markdown.NewParser(dataDir)

	// Get all tickets
	allTickets, err := findAllTickets(dataDir, parser)
	if err != nil {
		return fmt.Errorf("failed to find tickets: %w", err)
	}

	if len(allTickets) == 0 {
		fmt.Println("No tickets found.")
		return nil
	}

	// Filter based on argument
	var filterType string
	if len(args) > 0 {
		filterType = strings.ToLower(args[0])
	}

	switch filterType {
	case "epic":
		return listEpicsOnly(allTickets, ctxManager.Get())
	case "task":
		return listTasksOnly(allTickets, ctxManager.Get())
	case "subtask":
		return listSubtasksOnly(allTickets, ctxManager.Get())
	case "orphan":
		return listOrphanTasksOnly(allTickets, ctxManager.Get())
	case "spike":
		return listSpikesOnly(allTickets, ctxManager.Get())
	default:
		return listAllInTree(allTickets, ctxManager.Get())
	}
}

// listAllInTree shows all tickets in hierarchical tree structure
func listAllInTree(allTickets []types.Ticket, ctx *types.Context) error {
	// Group tickets by type
	var epics, tasks, subtasks, orphanTasks []types.Ticket

	for _, ticket := range allTickets {
		switch ticket.Type {
		case types.TicketTypeEpic:
			epics = append(epics, ticket)
		case types.TicketTypeTask, types.TicketTypeSpike:
			if ticket.EpicKey == "" {
				orphanTasks = append(orphanTasks, ticket)
			} else {
				tasks = append(tasks, ticket)
			}
		case types.TicketTypeSubtask:
			subtasks = append(subtasks, ticket)
		}
	}

	// Build tree structure
	tree := treepkg.New().Root("📋 All Tickets")
	tree.Enumerator(treepkg.RoundedEnumerator)

	// Add epics with their tasks and subtasks
	for _, epic := range epics {
		epicTree := buildEpicSubtree(epic, tasks, subtasks, ctx)
		tree.Child(epicTree)
	}

	// Add orphan tasks at the end
	if len(orphanTasks) > 0 {
		orphanTree := treepkg.New().Root("🏴‍☠️ Orphan Tasks")
		for _, orphanTask := range orphanTasks {
			taskTree := buildTaskSubtree(orphanTask, subtasks, ctx)
			orphanTree.Child(taskTree)
		}
		tree.Child(orphanTree)
	}

	fmt.Println(tree.String())
	return nil
}

// buildEpicSubtree builds a subtree for an epic with its tasks and subtasks
func buildEpicSubtree(epic types.Ticket, allTasks, allSubtasks []types.Ticket, ctx *types.Context) *treepkg.Tree {
	isEpicFocused := ctx.EpicKey == epic.Key && ctx.TaskKey == "" && ctx.SubtaskKey == ""
	epicTitle := formatTicketTitle("Epic", epic, isEpicFocused)
	epicTree := treepkg.New().Root(epicTitle)

	// Find tasks for this epic
	for _, task := range allTasks {
		if task.EpicKey == epic.Key {
			taskTree := buildTaskSubtree(task, allSubtasks, ctx)
			epicTree.Child(taskTree)
		}
	}

	return epicTree
}

// buildTaskSubtree builds a subtree for a task with its subtasks
func buildTaskSubtree(task types.Ticket, allSubtasks []types.Ticket, ctx *types.Context) *treepkg.Tree {
	isTaskFocused := ctx.TaskKey == task.Key && ctx.SubtaskKey == ""

	// Determine the correct type name for display
	var typeName string
	switch task.Type {
	case types.TicketTypeTask:
		typeName = "Task"
	case types.TicketTypeSpike:
		typeName = "Spike"
	default:
		typeName = "Task" // fallback
	}

	taskTitle := formatTicketTitle(typeName, task, isTaskFocused)
	taskTree := treepkg.New().Root(taskTitle)

	// Find subtasks for this task (spikes typically don't have subtasks, but check anyway)
	for _, subtask := range allSubtasks {
		if subtask.ParentKey == task.Key {
			isSubtaskFocused := ctx.SubtaskKey == subtask.Key
			subtaskTitle := formatTicketTitle("Subtask", subtask, isSubtaskFocused)
			taskTree.Child(subtaskTitle)
		}
	}

	return taskTree
}

// formatTicketTitle formats a ticket title with type, key, and focus indicator
func formatTicketTitle(ticketType string, ticket types.Ticket, isFocused bool) string {
	// Use the same styles as status_tree.go
	var style lipgloss.Style
	switch ticketType {
	case "Epic":
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#a259ec")).Bold(true)
	case "Task":
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b82f6")).Bold(true)
	case "Spike":
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b")).Bold(true) // Amber color for spikes
	case "Subtask":
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true)
	}

	// Clean title (remove Jira key if present)
	title := strings.TrimSpace(ticket.Title)
	if strings.Contains(title, "["+ticket.Key+"]") {
		title = strings.ReplaceAll(title, "["+ticket.Key+"]", "")
		title = strings.TrimSpace(title)
	}

	key := strings.TrimSpace(strings.ToUpper(ticket.Key))
	prefix := style.Render(ticketType)
	keyPart := fmt.Sprintf("[%s]", key)

	var desc string
	if isFocused {
		desc = lipgloss.NewStyle().Foreground(lipgloss.Color("#f4a259")).Bold(true).Render(title)
	} else {
		desc = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true).Render(title)
	}

	label := fmt.Sprintf("%s %s: %s", prefix, keyPart, desc)

	if isFocused {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffb300")).Bold(true).Render("*") + label
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true).Render(label)
}

// listEpicsOnly shows only epics
func listEpicsOnly(allTickets []types.Ticket, ctx *types.Context) error {
	fmt.Println("📋 Epics:")
	for _, ticket := range allTickets {
		if ticket.Type == types.TicketTypeEpic {
			isFocused := ctx.EpicKey == ticket.Key
			fmt.Println("  " + formatTicketTitle("Epic", ticket, isFocused))
		}
	}
	return nil
}

// listTasksOnly shows only tasks
func listTasksOnly(allTickets []types.Ticket, ctx *types.Context) error {
	fmt.Println("📋 Tasks:")
	for _, ticket := range allTickets {
		if ticket.Type == types.TicketTypeTask {
			isFocused := ctx.TaskKey == ticket.Key
			epicInfo := ""
			if ticket.EpicKey != "" {
				epicInfo = fmt.Sprintf(" (Epic: %s)", ticket.EpicKey)
			} else {
				epicInfo = " (Orphan)"
			}
			fmt.Println("  " + formatTicketTitle("Task", ticket, isFocused) + epicInfo)
		}
	}
	return nil
}

// listSubtasksOnly shows only subtasks
func listSubtasksOnly(allTickets []types.Ticket, ctx *types.Context) error {
	fmt.Println("📋 Subtasks:")
	for _, ticket := range allTickets {
		if ticket.Type == types.TicketTypeSubtask {
			isFocused := ctx.SubtaskKey == ticket.Key
			parentInfo := ""
			if ticket.ParentKey != "" {
				parentInfo = fmt.Sprintf(" (Task: %s)", ticket.ParentKey)
			}
			if ticket.EpicKey != "" {
				if parentInfo != "" {
					parentInfo += fmt.Sprintf(", Epic: %s", ticket.EpicKey)
				} else {
					parentInfo = fmt.Sprintf(" (Epic: %s)", ticket.EpicKey)
				}
			}
			fmt.Println("  " + formatTicketTitle("Subtask", ticket, isFocused) + parentInfo)
		}
	}
	return nil
}

// listSpikesOnly shows only spikes
func listSpikesOnly(allTickets []types.Ticket, ctx *types.Context) error {
	fmt.Println("🔍 Spikes:")
	for _, ticket := range allTickets {
		if ticket.Type == types.TicketTypeSpike {
			isFocused := ctx.TaskKey == ticket.Key // Spikes can be focused like tasks
			epicInfo := ""
			if ticket.EpicKey != "" {
				epicInfo = fmt.Sprintf(" (Epic: %s)", ticket.EpicKey)
			} else {
				epicInfo = " (Orphan)"
			}
			fmt.Println("  " + formatTicketTitle("Spike", ticket, isFocused) + epicInfo)
		}
	}
	return nil
}

// listOrphanTasksOnly shows only orphan tasks
func listOrphanTasksOnly(allTickets []types.Ticket, ctx *types.Context) error {
	fmt.Println("🏴‍☠️ Orphan Tasks:")
	found := false
	for _, ticket := range allTickets {
		if ticket.Type == types.TicketTypeTask && ticket.EpicKey == "" {
			found = true
			isFocused := ctx.TaskKey == ticket.Key
			fmt.Println("  " + formatTicketTitle("Task", ticket, isFocused))
		}
	}
	if !found {
		fmt.Println("  No orphan tasks found.")
	}
	return nil
}
