package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lunchboxsushi/jai/internal/types"
	"github.com/sashabaranov/go-openai"
)

// Provider defines the interface for AI providers
type Provider interface {
	Enrich(req *types.EnrichmentRequest) (*types.EnrichmentResponse, error)
}

// Service handles AI enrichment of tickets
type Service struct {
	providers map[string]Provider
	config    *types.Config
}

// NewService creates a new AI enrichment service
func NewService(config *types.Config) *Service {
	service := &Service{
		providers: make(map[string]Provider),
		config:    config,
	}

	// Register providers
	if config.AI.Provider == "openai" || config.AI.Provider == "" {
		service.providers["openai"] = NewOpenAIProvider(config)
	}
	// Add more providers here as needed
	// if config.AI.Provider == "anthropic" {
	//     service.providers["anthropic"] = NewAnthropicProvider(config)
	// }

	return service
}

// EnrichTicket enriches a ticket with AI-generated content
func (s *Service) EnrichTicket(req *types.EnrichmentRequest) (*types.EnrichmentResponse, error) {
	provider := s.config.AI.Provider
	if provider == "" {
		provider = "openai" // Default to OpenAI
	}

	p, exists := s.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported AI provider: %s", provider)
	}

	return p.Enrich(req)
}

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client *openai.Client
	config *types.Config
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config *types.Config) *OpenAIProvider {
	client := openai.NewClient(config.AI.APIKey)
	return &OpenAIProvider{
		client: client,
		config: config,
	}
}

// Enrich implements the Provider interface for OpenAI
func (p *OpenAIProvider) Enrich(req *types.EnrichmentRequest) (*types.EnrichmentResponse, error) {
	fmt.Printf("OpenAI: Starting enrichment with model %s\n", p.config.AI.Model)

	prompt := p.buildPrompt(req)
	fmt.Printf("OpenAI: Built prompt (length: %d characters)\n", len(prompt))

	resp, err := p.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: p.config.AI.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: p.getSystemPrompt(),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens:   p.config.AI.MaxTokens,
			Temperature: 0.7,
		},
	)

	if err != nil {
		fmt.Printf("OpenAI: Request failed: %v\n", err)

		// Provide more specific error messages based on the error type
		errStr := err.Error()
		if strings.Contains(errStr, "429") {
			if strings.Contains(errStr, "quota") || strings.Contains(errStr, "billing") {
				return nil, fmt.Errorf("OpenAI quota exceeded - please check your billing and usage limits: %w", err)
			} else if strings.Contains(errStr, "rate limit") {
				return nil, fmt.Errorf("OpenAI rate limit exceeded - too many requests, please wait and try again: %w", err)
			} else {
				return nil, fmt.Errorf("OpenAI 429 error - please check your account status and billing: %w", err)
			}
		} else if strings.Contains(errStr, "401") {
			return nil, fmt.Errorf("OpenAI authentication failed - please check your API key: %w", err)
		} else if strings.Contains(errStr, "403") {
			return nil, fmt.Errorf("OpenAI access forbidden - please check your account permissions: %w", err)
		}

		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	fmt.Printf("OpenAI: Request successful\n")

	if len(resp.Choices) == 0 {
		fmt.Printf("OpenAI: No choices in response\n")
		return nil, fmt.Errorf("no response from AI service")
	}

	content := resp.Choices[0].Message.Content
	fmt.Printf("OpenAI: Received response (length: %d characters)\n", len(content))
	fmt.Printf("OpenAI: Usage - Prompt: %d, Completion: %d, Total: %d\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)

	parsedResp, err := p.parseEnrichmentResponse(content)
	if err != nil {
		fmt.Printf("OpenAI: Failed to parse response: %v\n", err)
		return nil, err
	}

	fmt.Printf("OpenAI: Successfully parsed enrichment response\n")
	return parsedResp, nil
}

// buildPrompt builds the prompt for AI enrichment using configurable templates
func (p *OpenAIProvider) buildPrompt(req *types.EnrichmentRequest) string {
	// Load the prompt template
	template, err := p.loadPromptTemplate()
	if err != nil {
		fmt.Printf("OpenAI: Failed to load prompt template, using default: %v\n", err)
		return p.buildDefaultPrompt(req)
	}

	// PRE-PROCESS: Evaluate expressions in the raw content first
	processedRawContent := p.processContentExpressions(req.RawContent, req)

	// Replace template variables
	prompt := template
	prompt = strings.ReplaceAll(prompt, "{{TICKET_TYPE}}", string(req.Type))
	prompt = strings.ReplaceAll(prompt, "{{RAW_CONTENT}}", processedRawContent)

	// Build context string
	contextParts := []string{}
	if req.Context.EpicKey != "" {
		contextParts = append(contextParts, fmt.Sprintf("Epic: %s", req.Context.EpicKey))
	}
	if req.Context.TaskKey != "" {
		contextParts = append(contextParts, fmt.Sprintf("Task: %s", req.Context.TaskKey))
	}
	if req.Context.SubtaskKey != "" {
		contextParts = append(contextParts, fmt.Sprintf("Subtask: %s", req.Context.SubtaskKey))
	}
	contextStr := strings.Join(contextParts, " → ")
	if contextStr == "" {
		contextStr = "No context set"
	}
	prompt = strings.ReplaceAll(prompt, "{{CONTEXT}}", contextStr)

	// Extract and populate title if available
	title := p.extractTitleFromContent(processedRawContent)
	prompt = strings.ReplaceAll(prompt, "{{TITLE}}", title)

	fmt.Printf("OpenAI: Prompt before template expression processing:\n%s\n", prompt)

	// Process remaining {{expression}} patterns in the template (not in content)
	prompt = p.processTemplateExpressions(prompt)

	fmt.Printf("OpenAI: Final prompt after template expression processing:\n%s\n", prompt)

	return prompt
}

// loadPromptTemplate loads the prompt template from file or returns default
func (p *OpenAIProvider) loadPromptTemplate() (string, error) {
	// Check if custom template is configured
	if p.config.AI.PromptTemplate != "" {
		// Use absolute path if provided, otherwise relative to config dir
		templatePath := p.config.AI.PromptTemplate
		if !filepath.IsAbs(templatePath) {
			// Assume it's relative to ~/.jai/
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			templatePath = filepath.Join(home, ".jai", templatePath)
		}

		content, err := os.ReadFile(templatePath)
		if err != nil {
			return "", fmt.Errorf("failed to read template file %s: %w", templatePath, err)
		}
		return string(content), nil
	}

	// Try default template location
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	defaultPath := filepath.Join(home, ".jai", "templates", "enrichment_prompt.txt")
	content, err := os.ReadFile(defaultPath)
	if err != nil {
		return "", fmt.Errorf("no template found at %s: %w", defaultPath, err)
	}

	return string(content), nil
}

// processContentExpressions processes {{expression}} patterns within the raw content
// with full context preservation
func (p *OpenAIProvider) processContentExpressions(rawContent string, req *types.EnrichmentRequest) string {
	// Find all {{expression}} patterns in the raw content
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	matches := re.FindAllStringSubmatch(rawContent, -1)

	if len(matches) == 0 {
		return rawContent // No expressions to process
	}

	fmt.Printf("OpenAI: Found %d expressions to evaluate in raw content\n", len(matches))

	processedContent := rawContent

	for _, match := range matches {
		if len(match) >= 2 {
			fullMatch := match[0]
			expression := strings.TrimSpace(match[1])

			// Skip template variables (shouldn't be in content, but safety check)
			if expression == "TICKET_TYPE" || expression == "RAW_CONTENT" ||
				expression == "CONTEXT" || expression == "TITLE" {
				continue
			}

			// Evaluate the expression with full context of the problem
			result := p.evaluateExpressionWithContext(expression, rawContent, req)
			processedContent = strings.ReplaceAll(processedContent, fullMatch, result)
		}
	}

	return processedContent
}

// processTemplateExpressions processes {{expression}} patterns in the template (not content)
func (p *OpenAIProvider) processTemplateExpressions(prompt string) string {
	// Find all {{expression}} patterns
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	matches := re.FindAllStringSubmatch(prompt, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			fullMatch := match[0]
			expression := strings.TrimSpace(match[1])

			// Skip template variables we've already processed
			if expression == "TICKET_TYPE" || expression == "RAW_CONTENT" ||
				expression == "CONTEXT" || expression == "TITLE" {
				continue
			}

			// Evaluate the expression with AI
			result := p.evaluateExpression(expression)
			prompt = strings.ReplaceAll(prompt, fullMatch, result)
		}
	}

	return prompt
}

// evaluateExpression evaluates a single expression using AI
func (p *OpenAIProvider) evaluateExpression(expression string) string {
	// Simple AI call to evaluate the expression
	resp, err := p.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: p.config.AI.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a helpful assistant. Answer the user's request concisely and directly. If asked for a list, provide it in a simple format.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: expression,
				},
			},
			MaxTokens:   200, // Keep it short for expressions
			Temperature: 0.7,
		},
	)

	if err != nil {
		fmt.Printf("OpenAI: Failed to evaluate expression '%s': %v\n", expression, err)
		return fmt.Sprintf("[Error evaluating: %s]", expression)
	}

	if len(resp.Choices) == 0 {
		return fmt.Sprintf("[No response for: %s]", expression)
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	fmt.Printf("OpenAI: Evaluated expression '%s' → '%s'\n", expression, result)
	return result
}

// evaluateExpressionWithContext evaluates an expression with full context of the problem
func (p *OpenAIProvider) evaluateExpressionWithContext(expression, rawContent string, req *types.EnrichmentRequest) string {
	// Build context for the expression evaluation
	contextParts := []string{}

	// Add ticket type context
	contextParts = append(contextParts, fmt.Sprintf("This is for a %s ticket.", req.Type))

	// Add epic/task context if available
	if req.Context.EpicKey != "" {
		contextParts = append(contextParts, fmt.Sprintf("It's part of epic: %s", req.Context.EpicKey))
	}
	if req.Context.TaskKey != "" {
		contextParts = append(contextParts, fmt.Sprintf("It's related to task: %s", req.Context.TaskKey))
	}

	// Add the surrounding context from the raw content
	contextParts = append(contextParts, fmt.Sprintf("The full context is: %s", rawContent))

	contextStr := strings.Join(contextParts, " ")

	// Create a more detailed system prompt for contextual evaluation
	systemPrompt := `You are a helpful assistant evaluating expressions within the context of technical tasks. 
The user will provide you with an expression to evaluate, along with the full context of the problem.
Your response should be contextually appropriate and directly address the expression while considering the surrounding context.
If asked for a list, provide it in a simple, practical format relevant to the context.`

	userPrompt := fmt.Sprintf(`Please evaluate this expression: "%s"

Context: %s

Provide a direct, practical response that fits naturally within this context.`, expression, contextStr)

	resp, err := p.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: p.config.AI.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
			MaxTokens:   400, // Allow more tokens for contextual responses
			Temperature: 0.7,
		},
	)

	if err != nil {
		fmt.Printf("OpenAI: Failed to evaluate expression '%s' with context: %v\n", expression, err)
		return fmt.Sprintf("[Error evaluating: %s]", expression)
	}

	if len(resp.Choices) == 0 {
		return fmt.Sprintf("[No response for: %s]", expression)
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	fmt.Printf("OpenAI: Evaluated expression with context '%s' → '%s'\n", expression, result)
	return result
}

// extractTitleFromContent tries to extract a title from the raw content
func (p *OpenAIProvider) extractTitleFromContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// If first line looks like a title (short, no special chars), use it
		if len(firstLine) < 100 && !strings.Contains(firstLine, "\n") {
			return firstLine
		}
	}
	return "Untitled Task"
}

// buildDefaultPrompt builds a fallback prompt when template loading fails
func (p *OpenAIProvider) buildDefaultPrompt(req *types.EnrichmentRequest) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Ticket Type: %s", req.Type))

	if req.Context.EpicKey != "" {
		parts = append(parts, fmt.Sprintf("Epic Context: %s", req.Context.EpicKey))
	}

	if req.Context.TaskKey != "" {
		parts = append(parts, fmt.Sprintf("Parent Task: %s", req.Context.TaskKey))
	}

	parts = append(parts, "")
	parts = append(parts, "Raw Content:")
	parts = append(parts, req.RawContent)

	return strings.Join(parts, "\n")
}

// getSystemPrompt returns the system prompt for AI enrichment
func (p *OpenAIProvider) getSystemPrompt() string {
	// Since we're now using the full template as the user prompt,
	// we can simplify the system prompt
	return "You are a helpful AI assistant. Follow the instructions provided in the user message carefully."
}

// parseEnrichmentResponse parses the AI response into structured data
func (p *OpenAIProvider) parseEnrichmentResponse(content string) (*types.EnrichmentResponse, error) {
	fmt.Printf("OpenAI: Raw AI response to parse:\n%s\n", content)

	// First try to parse as JSON
	var jsonResp struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Summary     string   `json:"summary"`
		Labels      []string `json:"labels"`
		Priority    string   `json:"priority"`
	}

	// Try to find JSON in the response
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		jsonContent := content[jsonStart : jsonEnd+1]
		fmt.Printf("OpenAI: Extracted JSON content:\n%s\n", jsonContent)

		if err := json.Unmarshal([]byte(jsonContent), &jsonResp); err == nil {
			// Successfully parsed JSON
			resp := &types.EnrichmentResponse{
				Title:       jsonResp.Title,
				Description: jsonResp.Description,
				Summary:     jsonResp.Summary,
				Labels:      jsonResp.Labels,
				Priority:    jsonResp.Priority,
			}

			// Validate that we have essential fields
			if resp.Title == "" {
				resp.Title = p.extractTitleFromDescription(resp.Description)
			}

			fmt.Printf("OpenAI: Parsed JSON response - Title: %s, Description length: %d\n", resp.Title, len(resp.Description))
			return resp, nil
		} else {
			fmt.Printf("OpenAI: JSON parsing failed: %v\n", err)
		}
	}

	// Fallback to line-by-line parsing
	fmt.Printf("OpenAI: Using fallback parsing\n")
	return p.parseEnrichmentResponseFallback(content)
}

// extractTitleFromDescription extracts a title from the description if title is missing
func (p *OpenAIProvider) extractTitleFromDescription(description string) string {
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "**") && !strings.HasPrefix(line, "-") {
			// Use first non-empty, non-header line as title
			if len(line) > 100 {
				return line[:97] + "..."
			}
			return line
		}
	}
	return "Untitled Task"
}

// parseEnrichmentResponseFallback uses the original line-by-line parsing as fallback
func (p *OpenAIProvider) parseEnrichmentResponseFallback(content string) (*types.EnrichmentResponse, error) {
	lines := strings.Split(content, "\n")
	resp := &types.EnrichmentResponse{}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, `"title"`) {
			resp.Title = p.extractValue(line)
		} else if strings.Contains(line, `"description"`) {
			resp.Description = p.extractValue(line)
		} else if strings.Contains(line, `"summary"`) {
			resp.Summary = p.extractValue(line)
		} else if strings.Contains(line, `"priority"`) {
			resp.Priority = p.extractValue(line)
		} else if strings.Contains(line, `"labels"`) {
			// Handle array parsing
			resp.Labels = p.extractLabels(content)
		}
	}

	// Fallback: if we couldn't parse structured data, use the content as description
	if resp.Title == "" && resp.Description == "" {
		resp.Description = content
		resp.Title = "Generated Task"
	}

	// Ensure we always have a title
	if resp.Title == "" {
		resp.Title = p.extractTitleFromDescription(resp.Description)
	}

	return resp, nil
}

// extractValue extracts a value from a JSON-like line
func (p *OpenAIProvider) extractValue(line string) string {
	// Simple extraction - look for content between quotes after the colon
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return ""
	}

	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, `",`)
	return value
}

// extractLabels extracts labels from the content
func (p *OpenAIProvider) extractLabels(content string) []string {
	// Simple label extraction - look for array-like patterns
	re := regexp.MustCompile(`"([^"]+)"`)
	matches := re.FindAllStringSubmatch(content, -1)

	var labels []string
	for _, match := range matches {
		if len(match) > 1 {
			label := match[1]
			// Filter out common JSON keys
			if label != "title" && label != "description" && label != "summary" && label != "priority" && label != "labels" {
				labels = append(labels, label)
			}
		}
	}

	return labels
}
