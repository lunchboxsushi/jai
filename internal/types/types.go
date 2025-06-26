package types

import (
	"time"
)

// Context represents the current working context (epic/task focus)
type Context struct {
	EpicKey    string    `json:"epic_key,omitempty"`
	EpicID     string    `json:"epic_id,omitempty"`
	TaskKey    string    `json:"task_key,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
	SubtaskKey string    `json:"subtask_key,omitempty"`
	SubtaskID  string    `json:"subtask_id,omitempty"`
	Updated    time.Time `json:"updated"`
}

// Ticket represents a Jira ticket (epic, task, or sub-task)
type Ticket struct {
	Key          string                 `json:"key,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Type         TicketType             `json:"type"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description,omitempty"`
	RawContent   string                 `json:"raw_content,omitempty"`
	Enriched     string                 `json:"enriched,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Priority     string                 `json:"priority,omitempty"`
	Labels       []string               `json:"labels,omitempty"`
	Components   []string               `json:"components,omitempty"`
	Assignee     string                 `json:"assignee,omitempty"`
	Reporter     string                 `json:"reporter,omitempty"`
	Created      time.Time              `json:"created,omitempty"`
	Updated      time.Time              `json:"updated,omitempty"`
	DueDate      *time.Time             `json:"due_date,omitempty"`
	ParentKey    string                 `json:"parent_key,omitempty"`
	EpicKey      string                 `json:"epic_key,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
	LineNumber   int                    `json:"line_number,omitempty"` // Position in markdown file
}

// TicketType represents the type of Jira ticket
type TicketType string

const (
	TicketTypeEpic    TicketType = "epic"
	TicketTypeTask    TicketType = "task"
	TicketTypeSubtask TicketType = "subtask"
	TicketTypeSpike   TicketType = "spike"
)

// Config represents the application configuration
type Config struct {
	Jira struct {
		URL           string `yaml:"url" json:"url"`
		Username      string `yaml:"username" json:"username"`
		Token         string `yaml:"token" json:"token"`
		Project       string `yaml:"project" json:"project"`
		EpicLinkField string `yaml:"epic_link_field" json:"epic_link_field"`
	} `yaml:"jira" json:"jira"`

	AI struct {
		Provider       string `yaml:"provider" json:"provider"` // "openai", "anthropic", etc.
		APIKey         string `yaml:"api_key" json:"api_key"`
		Model          string `yaml:"model" json:"model"`
		MaxTokens      int    `yaml:"max_tokens" json:"max_tokens"`
		PromptTemplate string `yaml:"prompt_template" json:"prompt_template"` // Path to custom prompt template file
	} `yaml:"ai" json:"ai"`

	General struct {
		DataDir            string `yaml:"data_dir" json:"data_dir"`
		ReviewBeforeCreate bool   `yaml:"review_before_create" json:"review_before_create"`
		DefaultEditor      string `yaml:"default_editor" json:"default_editor"`
	} `yaml:"general" json:"general"`
}

// MarkdownFile represents a markdown file containing tickets
type MarkdownFile struct {
	Path    string   `json:"path"`
	Tickets []Ticket `json:"tickets"`
	Content string   `json:"content"`
}

// EnrichmentRequest represents a request to enrich a ticket
type EnrichmentRequest struct {
	RawContent string     `json:"raw_content"`
	Type       TicketType `json:"type"`
	Context    Context    `json:"context,omitempty"`
}

// EnrichmentResponse represents the response from AI enrichment
type EnrichmentResponse struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Summary     string   `json:"summary"`
	Labels      []string `json:"labels,omitempty"`
	Priority    string   `json:"priority,omitempty"`
}

// SyncOptions represents options for syncing with Jira
type SyncOptions struct {
	DryRun bool `json:"dry_run"`
	Diff   bool `json:"diff"`
	Status bool `json:"status"`
	Force  bool `json:"force"`
}
