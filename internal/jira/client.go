package jira

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	log.Printf("Creating Jira ticket - Type: %s, Title: %s", ticket.Type, ticket.Title)

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
		// Get the epic link custom field ID
		epicLinkField, err := c.GetEpicLinkField()
		if err != nil {
			log.Printf("Warning: Failed to get epic link field: %v", err)
		} else {
			// Set the epic link using custom fields
			if issue.Fields.Unknowns == nil {
				issue.Fields.Unknowns = make(map[string]interface{})
			}
			issue.Fields.Unknowns[epicLinkField] = ticket.EpicKey
			log.Printf("Setting epic link: %s = %s", epicLinkField, ticket.EpicKey)
		}
	}

	// Set parent for subtasks
	if ticket.Type == types.TicketTypeSubtask && ticket.ParentKey != "" {
		issue.Fields.Parent = &jira.Parent{
			Key: ticket.ParentKey,
		}
	}

	// Set priority
	if ticket.Priority != "" {
		// Jira expects priority as a string, not an object
		// Try setting it as a string directly
		// For now, let's skip priority setting to avoid the 400 error
		log.Printf("Note: Skipping priority setting to avoid Jira API error. Priority would be: %s", ticket.Priority)
	}

	// Log the issue fields being sent
	issueJson, _ := json.MarshalIndent(issue, "", "  ")
	log.Printf("Jira Issue Request Body:\n%s\n", string(issueJson))

	// Create the issue
	newIssue, resp, err := c.client.Issue.Create(issue)
	if err != nil {
		log.Printf("Jira API call failed with error: %v", err)

		// Try to read the response body for more details
		if resp != nil && resp.Body != nil {
			body, readErr := ioutil.ReadAll(resp.Body)
			if readErr == nil {
				log.Printf("Jira API Error Response Body:\n%s\n", string(body))
			} else {
				log.Printf("Failed to read response body: %v", readErr)
			}
			resp.Body.Close()
		}

		return nil, fmt.Errorf("failed to create Jira issue: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Jira ticket created successfully - Key: %s, ID: %s", newIssue.Key, newIssue.ID)

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

	// Set assignee
	if issue.Fields.Assignee != nil {
		ticket.Assignee = issue.Fields.Assignee.DisplayName
	}

	// Set reporter
	if issue.Fields.Reporter != nil {
		ticket.Reporter = issue.Fields.Reporter.DisplayName
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
	case "Spike":
		ticket.Type = types.TicketTypeSpike
	default:
		ticket.Type = types.TicketTypeTask
	}

	// Set priority
	if issue.Fields.Priority != nil {
		ticket.Priority = issue.Fields.Priority.Name
	}

	// Note: Epic linking would require custom field handling
	// For now, we'll skip this as it's complex to implement

	// Extract epic link if present
	if issue.Fields.Unknowns != nil {
		if epicLinkField, err := c.GetEpicLinkField(); err == nil {
			if epicKey, ok := issue.Fields.Unknowns[epicLinkField].(string); ok {
				ticket.EpicKey = epicKey
			}
		}
	}

	return ticket
}

// getIssueTypeName returns the Jira issue type name for our ticket type
func (c *Client) getIssueTypeName(ticketType types.TicketType) string {
	switch ticketType {
	case types.TicketTypeEpic:
		return "Epic"
	case types.TicketTypeSubtask:
		return "Sub-task"
	case types.TicketTypeSpike:
		return "Spike"
	default:
		return "Task"
	}
}

// GetEpicLinkField returns the custom field ID for epic links
func (c *Client) GetEpicLinkField() (string, error) {
	// Check if configured in config first
	if c.config.Jira.EpicLinkField != "" {
		return c.config.Jira.EpicLinkField, nil
	}

	// Common epic link field IDs for different Jira setups
	// These are the most common field IDs used for epic linking
	commonEpicFields := []string{
		"customfield_10014", // Most common
		"customfield_10008", // Alternative
		"customfield_10016", // Another common one
	}

	// For now, return the most common one
	// In a full implementation, you could query the Jira API to discover the correct field
	return commonEpicFields[0], nil
}
