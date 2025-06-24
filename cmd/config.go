package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage jai configuration including Jira and AI settings.

Examples:
  jai config init              # Initialize configuration
  jai config show              # Show current configuration
  jai config set jira.url https://company.atlassian.net`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return showConfig()
	}

	switch args[0] {
	case "init":
		return initConfigCmd()
	case "show":
		return showConfig()
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("usage: jai config set <key> <value>")
		}
		return setConfig(args[1], args[2])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// initConfigCmd initializes a new configuration file
func initConfigCmd() error {
	// Get config file path
	configPath := getConfigPath()

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create default config (without sensitive tokens)
	config := map[string]interface{}{
		"jira": map[string]interface{}{
			"url":      "",
			"username": "",
			"project":  "",
			// Note: token is NOT stored in config - use JAI_JIRA_TOKEN environment variable
		},
		"ai": map[string]interface{}{
			"provider":   "openai",
			"model":      "gpt-3.5-turbo",
			"max_tokens": 500,
			// Note: api_key is NOT stored in config - use JAI_AI_TOKEN environment variable
		},
		"general": map[string]interface{}{
			"data_dir":             "",
			"review_before_create": false,
			"default_editor":       "",
		},
	}

	// Write config file
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Configuration initialized at: %s\n", configPath)
	fmt.Println("Please edit the configuration file with your settings.")
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: API tokens are stored as environment variables only:")
	fmt.Println("   export JAI_JIRA_TOKEN=\"your-jira-api-token\"")
	fmt.Println("   export JAI_AI_TOKEN=\"your-openai-api-key\"")
	return nil
}

// showConfig shows the current configuration
func showConfig() error {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No configuration file found.")
		fmt.Println("Run 'jai config init' to create one.")
		return nil
	}

	fmt.Printf("Configuration file: %s\n\n", configPath)

	// Show Jira config
	fmt.Println("Jira Configuration:")
	fmt.Printf("  URL: %s\n", viper.GetString("jira.url"))
	fmt.Printf("  Username: %s\n", viper.GetString("jira.username"))
	fmt.Printf("  Project: %s\n", viper.GetString("jira.project"))

	// Check environment variable for Jira token
	jiraToken := os.Getenv("JAI_JIRA_TOKEN")
	if jiraToken != "" {
		fmt.Printf("  Token: %s (from environment)\n", maskString(jiraToken))
	} else {
		fmt.Println("  Token: NOT SET (set JAI_JIRA_TOKEN environment variable)")
	}

	fmt.Println()

	// Show AI config
	fmt.Println("AI Configuration:")
	fmt.Printf("  Provider: %s\n", viper.GetString("ai.provider"))
	fmt.Printf("  Model: %s\n", viper.GetString("ai.model"))
	fmt.Printf("  Max Tokens: %d\n", viper.GetInt("ai.max_tokens"))

	// Check environment variable for AI API key
	aiKey := os.Getenv("JAI_AI_TOKEN")
	if aiKey != "" {
		fmt.Printf("  API Key: %s (from environment)\n", maskString(aiKey))
	} else {
		fmt.Println("  API Key: NOT SET (set JAI_AI_TOKEN environment variable)")
	}

	fmt.Println()

	// Show general config
	fmt.Println("General Configuration:")
	fmt.Printf("  Data Directory: %s\n", viper.GetString("general.data_dir"))
	fmt.Printf("  Review Before Create: %t\n", viper.GetBool("general.review_before_create"))
	fmt.Printf("  Default Editor: %s\n", viper.GetString("general.default_editor"))

	return nil
}

// setConfig sets a configuration value
func setConfig(key, value string) error {
	// For now, we'll just show how to set it manually
	// In a full implementation, you'd want to actually modify the config file
	fmt.Printf("To set %s = %s, edit the configuration file manually:\n", key, value)
	fmt.Printf("  %s\n", getConfigPath())
	fmt.Println()
	fmt.Println("Or use environment variables:")
	fmt.Printf("  export JAI_%s=%s\n", strings.ToUpper(strings.ReplaceAll(key, ".", "_")), value)
	return nil
}

// getConfigPath returns the path to the configuration file
func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		return ".jai/config.yaml"
	}
	return filepath.Join(home, ".jai", "config.yaml")
}

// maskString masks sensitive strings for display
func maskString(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "****"
}
