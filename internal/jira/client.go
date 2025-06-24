package jira

import (
	"fmt"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/lunchboxsushi/jai/internal/types"
)

// Client handles Jira API interactions
type Client struct {
	client *jira.Client
	config *types.Config
}

// NewClient creates a new Jira client
func NewClient(config *types.Config) (*Client, error) {
	tp := jira.BasicAuthTransport{
		Username: config.Jira.Username,
		Password: config.Jira.Token,
	}

	client, err := jira.NewClient(tp.Client(), config.Jira.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

// CreateTicket creates a new Jira ticket
func (c *Client) CreateTicket(ticket *types.Ticket) (*types.Ticket, error) {
	issue := &jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: c.config.Jira.Project,
			},
			Summary:     ticket.Title,
			Description: ticket.Description,
			Type: jira.IssueType{
				Name: c.getIssueTypeName(ticket.Type),
			},
		},
	}

	// Set epic link if this is a task or subtask
	if ticket.Type == types.TicketTypeTask && ticket.EpicKey != "" {
		// For now, we'll skip epic linking as it requires custom field handling
		// In a full implementation, you'd need to map the epic link custom field
		fmt.Printf("Note: Epic linking to %s would be set here\n", ticket.EpicKey)
	}

	// Set parent for subtasks
	if ticket.Type == types.TicketTypeSubtask && ticket.ParentKey != "" {
		issue.Fields.Parent = &jira.Parent{
			Key: ticket.ParentKey,
		}
	}

	// Set labels
	if len(ticket.Labels) > 0 {
		issue.Fields.Labels = ticket.Labels
	}

	// Set priority
	if ticket.Priority != "" {
		issue.Fields.Priority = &jira.Priority{
			Name: ticket.Priority,
		}
	}

	// Create the issue
	newIssue, resp, err := c.client.Issue.Create(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira issue: %w", err)
	}
	defer resp.Body.Close()

	// Update the ticket with the created data
	ticket.Key = newIssue.Key
	ticket.ID = newIssue.ID
	ticket.Created = time.Now()
	ticket.Updated = time.Now()

	return ticket, nil
}

// GetTicket retrieves a ticket by key
func (c *Client) GetTicket(key string) (*types.Ticket, error) {
	issue, resp, err := c.client.Issue.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira issue: %w", err)
	}
	defer resp.Body.Close()

	return c.convertJiraIssue(issue), nil
}

// UpdateTicket updates an existing ticket
func (c *Client) UpdateTicket(ticket *types.Ticket) error {
	issue := &jira.Issue{
		Key: ticket.Key,
		Fields: &jira.IssueFields{
			Summary:     ticket.Title,
			Description: ticket.Description,
		},
	}

	if len(ticket.Labels) > 0 {
		issue.Fields.Labels = ticket.Labels
	}

	if ticket.Priority != "" {
		issue.Fields.Priority = &jira.Priority{
			Name: ticket.Priority,
		}
	}

	_, resp, err := c.client.Issue.Update(issue)
	if err != nil {
		return fmt.Errorf("failed to update Jira issue: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// SearchTickets searches for tickets using JQL
func (c *Client) SearchTickets(jql string) ([]*types.Ticket, error) {
	opts := &jira.SearchOptions{
		MaxResults: 100,
		StartAt:    0,
	}

	issues, resp, err := c.client.Issue.Search(jql, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search Jira issues: %w", err)
	}
	defer resp.Body.Close()

	var tickets []*types.Ticket
	for _, issue := range issues {
		tickets = append(tickets, c.convertJiraIssue(&issue))
	}

	return tickets, nil
}

// convertJiraIssue converts a Jira issue to our Ticket type
func (c *Client) convertJiraIssue(issue *jira.Issue) *types.Ticket {
	ticket := &types.Ticket{
		Key:         issue.Key,
		ID:          issue.ID,
		Title:       issue.Fields.Summary,
		Description: issue.Fields.Description,
		Status:      issue.Fields.Status.Name,
		Labels:      issue.Fields.Labels,
		Created:     time.Time(issue.Fields.Created),
		Updated:     time.Time(issue.Fields.Updated),
	}

	// Determine ticket type
	switch issue.Fields.Type.Name {
	case "Epic":
		ticket.Type = types.TicketTypeEpic
	case "Sub-task":
		ticket.Type = types.TicketTypeSubtask
		if issue.Fields.Parent != nil {
			ticket.ParentKey = issue.Fields.Parent.Key
		}
	default:
		ticket.Type = types.TicketTypeTask
	}

	// Set priority
	if issue.Fields.Priority != nil {
		ticket.Priority = issue.Fields.Priority.Name
	}

	// Note: Epic linking would require custom field handling
	// For now, we'll skip this as it's complex to implement

	return ticket
}

// getIssueTypeName returns the Jira issue type name for our ticket type
func (c *Client) getIssueTypeName(ticketType types.TicketType) string {
	switch ticketType {
	case types.TicketTypeEpic:
		return "Epic"
	case types.TicketTypeSubtask:
		return "Sub-task"
	default:
		return "Task"
	}
}

// GetEpicLinkField returns the custom field ID for epic links
func (c *Client) GetEpicLinkField() (string, error) {
	// This would typically be configured or discovered via API
	// For now, we'll use a common default
	return "customfield_10014", nil
}
