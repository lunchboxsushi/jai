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
	"github.com/spf13/viper"
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("202")).Bold(true)
	// Epics: purple
	epicStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a259ec")).Bold(true)
	// Tasks: lighter blue for better readability
	taskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b82f6")).Bold(true)
	// Subtasks: light blue (matches screenshot)
	subtaskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true)
	// Description: warm orange for focused, white for others
	descStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f4a259")).Bold(true)
	whiteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
	// Bright orange asterisk for focused item
	focusAsteriskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffb300")).Bold(true)
)

func renderStatusTree(ctxManager *context.Manager) error {
	dataDir := viper.GetString("general.data_dir")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share", "jai")
	}

	parser := markdown.NewParser(dataDir)
	currentCtx := ctxManager.Get()

	if !ctxManager.HasEpic() {
		fmt.Println("No epic in context. Use 'jai focus <epic-key>' to set one.")
		return nil
	}

	allTickets, err := findAllTickets(dataDir, parser)
	if err != nil {
		return err
	}

	var rootEpic *types.Ticket
	for i, t := range allTickets {
		if t.Key == currentCtx.EpicKey {
			rootEpic = &allTickets[i]
			break
		}
	}

	if rootEpic == nil {
		fmt.Printf("Focused epic '%s' not found in any markdown file.\n", currentCtx.EpicKey)
		return nil
	}

	tasks := findChildTasks(rootEpic.Key, allTickets)
	treeRoot := buildTree(rootEpic, tasks, allTickets, currentCtx)

	// Use rounded enumerator for a more visually distinct tree
	treeRoot.Enumerator(treepkg.RoundedEnumerator)

	fmt.Println(treeRoot.String())

	return nil
}

func findChildTasks(epicKey string, allTickets []types.Ticket) []*types.Ticket {
	var tasks []*types.Ticket
	for i, t := range allTickets {
		if t.Type == types.TicketTypeTask && t.EpicKey == epicKey {
			tasks = append(tasks, &allTickets[i])
		}
	}
	return tasks
}

func findChildSubtasks(taskKey string, allTickets []types.Ticket) []*types.Ticket {
	var subtasks []*types.Ticket
	for i, t := range allTickets {
		if t.Type == types.TicketTypeSubtask && t.ParentKey == taskKey {
			subtasks = append(subtasks, &allTickets[i])
		}
	}
	return subtasks
}

func buildTree(epic *types.Ticket, tasks []*types.Ticket, allTickets []types.Ticket, ctx *types.Context) *treepkg.Tree {
	parser := markdown.NewParser("")
	// Only deepest focus gets [FOCUSED]
	focusLevel := ""
	if ctx.SubtaskKey != "" {
		focusLevel = "subtask"
	} else if ctx.TaskKey != "" {
		focusLevel = "task"
	} else if ctx.EpicKey != "" {
		focusLevel = "epic"
	}

	epictitle := formatNodeTitle("Epic", parser.RemoveJiraKey(epic.Title), epic.Key, focusLevel == "epic" && ctx.EpicKey == epic.Key, epicStyle)
	tree := treepkg.New().Root(epictitle)

	for _, task := range tasks {
		isTaskFocused := focusLevel == "task" && ctx.TaskKey == task.Key
		taskTitle := formatNodeTitle("Task", parser.RemoveJiraKey(task.Title), task.Key, isTaskFocused, taskStyle)
		taskTree := treepkg.New().Root(taskTitle)

		subtasks := findChildSubtasks(task.Key, allTickets)
		for _, subtask := range subtasks {
			isSubFocused := focusLevel == "subtask" && ctx.SubtaskKey == subtask.Key
			subtaskTitle := formatNodeTitle("Subtask", parser.RemoveJiraKey(subtask.Title), subtask.Key, isSubFocused, subtaskStyle)
			taskTree.Child(subtaskTitle)
		}
		tree.Child(taskTree)
	}

	return tree
}

func formatNodeTitle(kind, title, key string, isFocused bool, style lipgloss.Style) string {
	title = strings.TrimSpace(title)
	key = strings.TrimSpace(strings.ToUpper(key))
	var prefix string
	switch kind {
	case "Epic":
		prefix = epicStyle.Render("Epic")
	case "Task":
		prefix = taskStyle.Render("Task")
	case "Subtask":
		prefix = subtaskStyle.Render("Subtask")
	}
	keyPart := fmt.Sprintf("[%s]", key)
	var desc string
	if isFocused {
		desc = descStyle.Render(title)
	} else {
		desc = whiteStyle.Render(title)
	}
	label := fmt.Sprintf("%s %s: %s", prefix, keyPart, desc)
	if isFocused {
		return focusAsteriskStyle.Render("*") + label
	}
	// Dim non-focused items
	return dimStyle.Render(label)
}
