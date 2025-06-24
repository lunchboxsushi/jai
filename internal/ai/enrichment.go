package ai

import (
	"context"
	"fmt"
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
	prompt := p.buildPrompt(req)

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
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI service")
	}

	content := resp.Choices[0].Message.Content
	return p.parseEnrichmentResponse(content)
}

// buildPrompt builds the prompt for AI enrichment
func (p *OpenAIProvider) buildPrompt(req *types.EnrichmentRequest) string {
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
	return `You are an expert at converting raw developer notes into polished, manager-friendly Jira ticket descriptions.

Your task is to enrich raw task descriptions with:
1. A clear, concise title
2. A detailed description that explains the what, why, and how
3. A brief summary for quick understanding
4. Appropriate labels and priority

Guidelines:
- Keep titles under 100 characters
- Make descriptions actionable and clear
- Use technical language appropriately
- Consider the context (epic, parent task)
- Suggest relevant labels and priority levels
- Maintain the original intent while making it professional

Respond in the following JSON format:
{
  "title": "Clear, concise title",
  "description": "Detailed description explaining what needs to be done, why it's important, and how it should be approached",
  "summary": "Brief summary for quick understanding",
  "labels": ["label1", "label2"],
  "priority": "High|Medium|Low"
}`
}

// parseEnrichmentResponse parses the AI response into structured data
func (p *OpenAIProvider) parseEnrichmentResponse(content string) (*types.EnrichmentResponse, error) {
	// For now, we'll do a simple parsing approach
	// In a production system, you'd want to use proper JSON parsing with fallbacks

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
