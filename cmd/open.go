package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var openCmd = &cobra.Command{
	Use:   "open [level]",
	Short: "Open a Jira ticket in the browser based on current context",
	Long: `Open a Jira ticket in the browser based on current context hierarchy.

Examples:
  jai open           # Open the current focus item (deepest level)
  jai open subtask   # Open the current subtask (same as above if focused on subtask)
  jai open task      # Open the current task (or parent task if focused on subtask)
  jai open epic      # Open the current epic (or parent epic if focused on task/subtask)

The command understands your current focus context and navigates the hierarchy:
- If focused on a subtask: "task" opens the parent task, "epic" opens the parent epic
- If focused on a task: "epic" opens the parent epic
- If focused on an epic: only "epic" is valid`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
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

	currentCtx := ctxManager.Get()

	// Check if we have any context
	if !ctxManager.HasEpic() && !ctxManager.HasTask() && !ctxManager.HasSubtask() {
		return fmt.Errorf("no context set. Use 'jai focus' to set focus on a ticket first")
	}

	// Get Jira URL from config
	jiraURL := viper.GetString("jira.url")
	if jiraURL == "" {
		return fmt.Errorf("Jira URL not configured. Please run 'jai init' to configure Jira settings")
	}

	// Determine which ticket to open based on args and current context
	var ticketKey string
	var ticketType string

	if len(args) == 0 {
		// Open the current focus item (deepest level)
		ticketKey, ticketType = getCurrentFocusTicket(currentCtx)
	} else {
		// Open specific level within current context
		level := strings.ToLower(args[0])
		var err error
		ticketKey, ticketType, err = getTicketByLevel(currentCtx, level)
		if err != nil {
			return err
		}
	}

	if ticketKey == "" {
		return fmt.Errorf("no ticket found to open")
	}

	// Construct Jira URL and open in browser
	ticketURL := fmt.Sprintf("%s/browse/%s", strings.TrimRight(jiraURL, "/"), ticketKey)

	fmt.Printf("Opening %s %s in browser: %s\n", ticketType, ticketKey, ticketURL)

	if err := openBrowser(ticketURL); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

// getCurrentFocusTicket returns the current focus ticket (deepest level)
func getCurrentFocusTicket(ctx *types.Context) (string, string) {
	if ctx.SubtaskKey != "" {
		return ctx.SubtaskKey, "subtask"
	}
	if ctx.TaskKey != "" {
		return ctx.TaskKey, "task"
	}
	if ctx.EpicKey != "" {
		return ctx.EpicKey, "epic"
	}
	return "", ""
}

// getTicketByLevel returns the ticket key for the specified level within current context
func getTicketByLevel(ctx *types.Context, level string) (string, string, error) {
	switch level {
	case "subtask":
		if ctx.SubtaskKey == "" {
			return "", "", fmt.Errorf("no subtask in current context")
		}
		return ctx.SubtaskKey, "subtask", nil

	case "task":
		if ctx.TaskKey == "" {
			return "", "", fmt.Errorf("no task in current context")
		}
		return ctx.TaskKey, "task", nil

	case "epic":
		if ctx.EpicKey == "" {
			return "", "", fmt.Errorf("no epic in current context")
		}
		return ctx.EpicKey, "epic", nil

	default:
		return "", "", fmt.Errorf("invalid level '%s'. Valid levels are: epic, task, subtask", level)
	}
}

// openBrowser opens the given URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
