package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lunchboxsushi/jai/internal/context"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var unfocusCmd = &cobra.Command{
	Use:   "unfocus",
	Short: "Clear current context",
	Long: `Clear the current epic and task context.

Examples:
  jai unfocus              # Clear all context`,
	RunE: runUnfocus,
}

func init() {
	rootCmd.AddCommand(unfocusCmd)
}

func runUnfocus(cmd *cobra.Command, args []string) error {
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

	// Clear the context
	if err := ctxManager.Clear(); err != nil {
		return fmt.Errorf("failed to clear context: %w", err)
	}

	fmt.Println("Context cleared")
	return nil
}
