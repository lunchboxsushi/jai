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
	epicStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	taskStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	subtaskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
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
	epictitle := formatNodeTitle(parser.RemoveJiraKey(epic.Title), epic.Key, ctx.EpicKey == epic.Key, epicStyle)
	tree := treepkg.New().Root(epictitle)

	for _, task := range tasks {
		isFocused := ctx.TaskKey == task.Key
		taskTitle := formatNodeTitle(parser.RemoveJiraKey(task.Title), task.Key, isFocused, taskStyle)
		taskTree := treepkg.New().Root(taskTitle)

		subtasks := findChildSubtasks(task.Key, allTickets)
		for _, subtask := range subtasks {
			isSubFocused := ctx.SubtaskKey == subtask.Key
			subtaskTitle := formatNodeTitle(parser.RemoveJiraKey(subtask.Title), subtask.Key, isSubFocused, subtaskStyle)
			taskTree.Child(subtaskTitle)
		}
		tree.Child(taskTree)
	}

	return tree
}

func formatNodeTitle(title, key string, isFocused bool, style lipgloss.Style) string {
	if strings.TrimSpace(title) == "" {
		title = key
	} else {
		title = fmt.Sprintf("%s [%s]", title, key)
	}

	if isFocused {
		return focusedStyle.Render(title + " [FOCUSED]")
	}
	return style.Render(title)
}
