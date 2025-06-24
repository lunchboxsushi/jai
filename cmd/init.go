package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard for JAI",
	Long: `Interactive setup wizard that guides you through initial JAI configuration.

This command will:
1. Create the configuration directory and file
2. Prompt for Jira settings (URL, username, project)
3. Prompt for AI settings (provider, model)
4. Set up environment variable instructions
5. Create initial data directories

Sensitive values (API tokens) are handled via environment variables only.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Welcome to JAI Setup!")
	fmt.Println("This wizard will help you configure JAI for first use.")
	fmt.Println()

	// Get config file path
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Initialize configuration
	config := make(map[string]interface{})

	// Jira Configuration
	fmt.Println("📋 Jira Configuration")
	fmt.Println("----------------------")

	jiraURL := promptForInput("Jira Cloud URL (e.g., https://company.atlassian.net): ", "")
	if jiraURL == "" {
		return fmt.Errorf("Jira URL is required")
	}

	jiraUsername := promptForInput("Jira username/email: ", "")
	if jiraUsername == "" {
		return fmt.Errorf("Jira username is required")
	}

	jiraProject := promptForInput("Default Jira project key (e.g., PROJ): ", "")
	if jiraProject == "" {
		return fmt.Errorf("Jira project key is required")
	}

	config["jira"] = map[string]interface{}{
		"url":      jiraURL,
		"username": jiraUsername,
		"project":  jiraProject,
		// Note: token is NOT stored in config - use environment variable
	}

	fmt.Println()

	// AI Configuration
	fmt.Println("🤖 AI Configuration")
	fmt.Println("-------------------")

	aiProvider := promptForInput("AI provider (openai/anthropic) [openai]: ", "openai")
	if aiProvider == "" {
		aiProvider = "openai"
	}

	aiModel := promptForInput("AI model [gpt-3.5-turbo]: ", "gpt-3.5-turbo")
	if aiModel == "" {
		aiModel = "gpt-3.5-turbo"
	}

	maxTokens := promptForInput("Max tokens for AI responses [500]: ", "500")
	if maxTokens == "" {
		maxTokens = "500"
	}

	config["ai"] = map[string]interface{}{
		"provider":   aiProvider,
		"model":      aiModel,
		"max_tokens": maxTokens,
		// Note: api_key is NOT stored in config - use environment variable
	}

	fmt.Println()

	// General Configuration
	fmt.Println("⚙️ General Configuration")
	fmt.Println("------------------------")

	defaultEditor := promptForInput("Default editor for task drafting [vim]: ", "vim")
	if defaultEditor == "" {
		defaultEditor = "vim"
	}

	reviewBeforeCreate := promptForInput("Ask for review before creating Jira tickets? (y/n) [n]: ", "n")
	reviewBeforeCreateBool := strings.ToLower(reviewBeforeCreate) == "y"

	config["general"] = map[string]interface{}{
		"data_dir":             "",
		"review_before_create": reviewBeforeCreateBool,
		"default_editor":       defaultEditor,
	}

	fmt.Println()

	// Write configuration file
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create data directory
	dataDir := getDefaultDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create tickets directory
	ticketsDir := filepath.Join(dataDir, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tickets directory: %w", err)
	}

	// Create templates directory
	templatesDir := filepath.Join(dataDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	fmt.Println("✅ Configuration created successfully!")
	fmt.Printf("📁 Config file: %s\n", configPath)
	fmt.Printf("📁 Data directory: %s\n", dataDir)
	fmt.Println()

	// Show environment variable setup
	fmt.Println("🔐 Environment Variables Required")
	fmt.Println("=================================")
	fmt.Println("For security, API tokens are stored as environment variables only.")
	fmt.Println()
	fmt.Println("Add these to your shell profile (~/.bashrc, ~/.zshrc, etc.):")
	fmt.Println()
	fmt.Printf("export JAI_JIRA_TOKEN=\"your-jira-api-token\"\n")
	fmt.Printf("export JAI_AI_TOKEN=\"your-openai-api-key\"\n")
	fmt.Println()
	fmt.Println("To get your Jira API token:")
	fmt.Println("1. Go to https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Println("2. Create a new API token")
	fmt.Println("3. Copy the token and add it to your environment")
	fmt.Println()
	fmt.Println("To get your OpenAI API key:")
	fmt.Println("1. Go to https://platform.openai.com/api-keys")
	fmt.Println("2. Create a new API key")
	fmt.Println("3. Copy the key and add it to your environment")
	fmt.Println()

	// Test configuration
	fmt.Println("🧪 Testing Configuration")
	fmt.Println("========================")
	fmt.Println("Run 'jai status' to verify your configuration.")
	fmt.Println("Run 'jai epic \"test-epic\"' to create your first epic.")
	fmt.Println()

	return nil
}

// promptForInput prompts for user input with a default value
func promptForInput(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	if defaultValue != "" {
		fmt.Printf("%s[%s]: ", prompt, defaultValue)
	} else {
		fmt.Print(prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}

	return input
}

// getDefaultDataDir returns the default data directory
func getDefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".local/share/jai"
	}
	return filepath.Join(home, ".local", "share", "jai")
}
