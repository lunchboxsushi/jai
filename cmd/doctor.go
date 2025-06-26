package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lunchboxsushi/jai/internal/ai"
	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and validate JAI configuration and connectivity",
	Long: `Run diagnostics to check JAI configuration, OpenAI connectivity, and system health.

This command will:
- Validate configuration files
- Check OpenAI API connectivity
- Test AI enrichment functionality
- Verify environment variables
- Check data directory permissions`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 JAI Doctor - System Diagnostics")
	fmt.Println("==================================")

	// Check 1: Environment Variables
	fmt.Println("\n1. Checking Environment Variables...")
	checkEnvironmentVariables()

	// Check 2: Configuration
	fmt.Println("\n2. Checking Configuration...")
	if err := checkConfiguration(); err != nil {
		return err
	}

	// Check 3: OpenAI Connectivity
	fmt.Println("\n3. Checking OpenAI Connectivity...")
	if err := checkOpenAIConnectivity(); err != nil {
		return err
	}

	// Check 4: AI Enrichment Test
	fmt.Println("\n4. Testing AI Enrichment...")
	if err := testAIEnrichment(); err != nil {
		return err
	}

	// Check 5: Data Directory
	fmt.Println("\n5. Checking Data Directory...")
	if err := checkDataDirectory(); err != nil {
		return err
	}

	fmt.Println("\n✅ All checks completed!")
	return nil
}

func checkEnvironmentVariables() {
	aiToken := os.Getenv("JAI_AI_TOKEN")
	jiraToken := os.Getenv("JAI_JIRA_TOKEN")

	if aiToken == "" {
		fmt.Println("❌ JAI_AI_TOKEN not set")
	} else {
		fmt.Printf("✅ JAI_AI_TOKEN set (length: %d)\n", len(aiToken))
	}

	if jiraToken == "" {
		fmt.Println("⚠️  JAI_JIRA_TOKEN not set (Jira integration will be disabled)")
	} else {
		fmt.Printf("✅ JAI_JIRA_TOKEN set (length: %d)\n", len(jiraToken))
	}
}

func checkConfiguration() error {
	// Check if config file exists
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		fmt.Println("⚠️  No config file found, using defaults")
	} else {
		fmt.Printf("✅ Config file: %s\n", configFile)
	}

	// Check AI configuration
	aiProvider := viper.GetString("ai.provider")
	aiModel := viper.GetString("ai.model")
	aiMaxTokens := viper.GetInt("ai.max_tokens")

	fmt.Printf("AI Provider: %s\n", aiProvider)
	fmt.Printf("AI Model: %s\n", aiModel)
	fmt.Printf("AI Max Tokens: %d\n", aiMaxTokens)

	// Check general configuration
	dataDir := viper.GetString("general.data_dir")
	defaultEditor := viper.GetString("general.default_editor")
	reviewBeforeCreate := viper.GetBool("general.review_before_create")

	fmt.Printf("Data Directory: %s\n", dataDir)
	fmt.Printf("Default Editor: %s\n", defaultEditor)
	fmt.Printf("Review Before Create: %t\n", reviewBeforeCreate)

	return nil
}

func checkOpenAIConnectivity() error {
	apiKey := os.Getenv("JAI_AI_TOKEN")
	if apiKey == "" {
		return fmt.Errorf("JAI_AI_TOKEN not set")
	}

	// Create OpenAI client
	client := openai.NewClient(apiKey)

	// Test with a simple request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Testing OpenAI API with a simple request...")

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "gpt-3.5-turbo",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hello! Please respond with 'JAI connectivity test successful'",
				},
			},
			MaxTokens:   10,
			Temperature: 0,
		},
	)

	if err != nil {
		fmt.Printf("❌ OpenAI API test failed: %v\n", err)

		// Provide more specific error information
		errStr := err.Error()
		if strings.Contains(errStr, "429") {
			if strings.Contains(errStr, "quota") || strings.Contains(errStr, "billing") {
				fmt.Println("💡 This appears to be a quota/billing issue. Please check:")
				fmt.Println("   - Your OpenAI account billing status")
				fmt.Println("   - Your usage limits (free tier has monthly limits)")
				fmt.Println("   - Your account verification status")
			} else {
				fmt.Println("💡 This appears to be a rate limiting issue. Please wait and try again.")
			}
		} else if strings.Contains(errStr, "401") {
			fmt.Println("💡 This appears to be an authentication issue. Please check:")
			fmt.Println("   - Your API key is correct")
			fmt.Println("   - Your API key is from the right account")
		}

		return err
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response from OpenAI API")
	}

	content := resp.Choices[0].Message.Content
	fmt.Printf("✅ OpenAI API test successful: %s\n", content)
	fmt.Printf("   Model used: %s\n", resp.Model)
	fmt.Printf("   Usage - Prompt tokens: %d, Completion tokens: %d, Total tokens: %d\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)

	return nil
}

func testAIEnrichment() error {
	// Create a test ticket
	testTicket := &types.Ticket{
		Type:       types.TicketTypeTask,
		Title:      "Test Task",
		RawContent: "Fix the login bug that occurs when users try to authenticate with OAuth",
		Created:    time.Now(),
		Updated:    time.Now(),
	}

	testCtx := &types.Context{
		EpicKey: "TEST-EPIC",
		TaskKey: "",
	}

	// Get AI config
	aiConfig := &types.Config{
		AI: struct {
			Provider       string `yaml:"provider" json:"provider"`
			APIKey         string `yaml:"api_key" json:"api_key"`
			Model          string `yaml:"model" json:"model"`
			MaxTokens      int    `yaml:"max_tokens" json:"max_tokens"`
			PromptTemplate string `yaml:"prompt_template" json:"prompt_template"`
		}{
			Provider:       viper.GetString("ai.provider"),
			APIKey:         os.Getenv("JAI_AI_TOKEN"),
			Model:          viper.GetString("ai.model"),
			MaxTokens:      viper.GetInt("ai.max_tokens"),
			PromptTemplate: viper.GetString("ai.prompt_template"),
		},
	}

	if aiConfig.AI.Model == "" {
		aiConfig.AI.Model = "gpt-3.5-turbo"
	}

	if aiConfig.AI.MaxTokens == 0 {
		aiConfig.AI.MaxTokens = 500
	}

	fmt.Printf("Testing AI enrichment with model: %s\n", aiConfig.AI.Model)

	// Create AI service
	aiService := ai.NewService(aiConfig)

	// Create enrichment request
	req := &types.EnrichmentRequest{
		RawContent: testTicket.RawContent,
		Type:       testTicket.Type,
		Context:    *testCtx,
	}

	// Test enrichment
	resp, err := aiService.EnrichTicket(req)
	if err != nil {
		fmt.Printf("❌ AI enrichment test failed: %v\n", err)
		return err
	}

	fmt.Printf("✅ AI enrichment test successful\n")
	fmt.Printf("   Original title: %s\n", testTicket.Title)
	fmt.Printf("   Enriched title: %s\n", resp.Title)
	fmt.Printf("   Description length: %d characters\n", len(resp.Description))
	fmt.Printf("   Labels: %v\n", resp.Labels)
	fmt.Printf("   Priority: %s\n", resp.Priority)

	return nil
}

func checkDataDirectory() error {
	dataDir := viper.GetString("general.data_dir")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = fmt.Sprintf("%s/.local/share/jai", home)
	}

	fmt.Printf("Data directory: %s\n", dataDir)

	// Check if directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		fmt.Printf("⚠️  Data directory does not exist: %s\n", dataDir)
		fmt.Println("   It will be created when you first use JAI")
	} else if err != nil {
		return fmt.Errorf("failed to check data directory: %w", err)
	} else {
		fmt.Printf("✅ Data directory exists: %s\n", dataDir)
	}

	// Check if we can write to the directory
	testFile := fmt.Sprintf("%s/.test_write", dataDir)
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		fmt.Printf("❌ Cannot write to data directory: %v\n", err)
		return err
	}

	// Clean up test file
	os.Remove(testFile)
	fmt.Printf("✅ Data directory is writable\n")

	return nil
}
