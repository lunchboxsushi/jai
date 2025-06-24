package markdown

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lunchboxsushi/jai/internal/types"
)

// Parser handles parsing and writing markdown files
type Parser struct {
	dataDir string
}

// NewParser creates a new markdown parser
func NewParser(dataDir string) *Parser {
	return &Parser{
		dataDir: dataDir,
	}
}

// ParseFile parses a markdown file and extracts tickets
func (p *Parser) ParseFile(filePath string) (*types.MarkdownFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	tickets := p.extractTickets(content, filePath)

	return &types.MarkdownFile{
		Path:    filePath,
		Tickets: tickets,
		Content: content,
	}, nil
}

// WriteFile writes tickets to a markdown file
func (p *Parser) WriteFile(filePath string, tickets []types.Ticket) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	content := p.generateMarkdown(tickets)
	return os.WriteFile(filePath, []byte(content), 0644)
}

// extractTickets extracts tickets from markdown content
func (p *Parser) extractTickets(content, filePath string) []types.Ticket {
	var tickets []types.Ticket
	scanner := bufio.NewScanner(strings.NewReader(content))

	lineNum := 0
	currentTicket := &types.Ticket{}
	inTicket := false
	var lines []string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for ticket headers
		if p.isTicketHeader(line) {
			// Save previous ticket if exists
			if inTicket {
				currentTicket.RawContent = strings.TrimSpace(strings.Join(lines, "\n"))
				tickets = append(tickets, *currentTicket)
			}

			// Start new ticket
			currentTicket = p.parseTicketHeader(line, lineNum)
			inTicket = true
			lines = []string{}
			continue
		}

		if inTicket {
			lines = append(lines, line)
		}
	}

	// Don't forget the last ticket
	if inTicket {
		currentTicket.RawContent = strings.TrimSpace(strings.Join(lines, "\n"))
		tickets = append(tickets, *currentTicket)
	}

	return tickets
}

// isTicketHeader checks if a line is a ticket header
func (p *Parser) isTicketHeader(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "# epic:") ||
		strings.HasPrefix(line, "## task:") ||
		strings.HasPrefix(line, "### subtask:")
}

// parseTicketHeader parses a ticket header line
func (p *Parser) parseTicketHeader(line string, lineNum int) *types.Ticket {
	line = strings.TrimSpace(line)

	ticket := &types.Ticket{
		LineNumber: lineNum,
	}

	// Extract title and type
	if strings.HasPrefix(line, "# epic:") {
		ticket.Type = types.TicketTypeEpic
		ticket.Title = strings.TrimSpace(strings.TrimPrefix(line, "# epic:"))
	} else if strings.HasPrefix(line, "## task:") {
		ticket.Type = types.TicketTypeTask
		ticket.Title = strings.TrimSpace(strings.TrimPrefix(line, "## task:"))
	} else if strings.HasPrefix(line, "### subtask:") {
		ticket.Type = types.TicketTypeSubtask
		ticket.Title = strings.TrimSpace(strings.TrimPrefix(line, "### subtask:"))
	}

	// Extract Jira key if present
	if key := p.extractJiraKey(ticket.Title); key != "" {
		ticket.Key = key
	}

	return ticket
}

// extractJiraKey extracts a Jira key from text
func (p *Parser) extractJiraKey(text string) string {
	// Look for patterns like "PROJ-123" or "[PROJ-123]"
	re := regexp.MustCompile(`\[?([A-Z]+-\d+)\]?`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// generateMarkdown generates markdown content from tickets
func (p *Parser) generateMarkdown(tickets []types.Ticket) string {
	var lines []string

	for _, ticket := range tickets {
		// Add header
		header := p.generateHeader(ticket)
		lines = append(lines, header)

		// Add raw content
		if ticket.RawContent != "" {
			lines = append(lines, ticket.RawContent)
		}

		// Add enriched content if available
		if ticket.Enriched != "" {
			lines = append(lines, "")
			lines = append(lines, "---")
			lines = append(lines, "*Enriched:*")
			lines = append(lines, ticket.Enriched)
		}

		// Add metadata if available
		if ticket.Key != "" || ticket.Status != "" || ticket.Priority != "" {
			lines = append(lines, "")
			lines = append(lines, "---")
			lines = append(lines, "*Metadata:*")
			if ticket.Key != "" {
				lines = append(lines, fmt.Sprintf("- Key: %s", ticket.Key))
			}
			if ticket.Status != "" {
				lines = append(lines, fmt.Sprintf("- Status: %s", ticket.Status))
			}
			if ticket.Priority != "" {
				lines = append(lines, fmt.Sprintf("- Priority: %s", ticket.Priority))
			}
		}

		lines = append(lines, "")
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// generateHeader generates a markdown header for a ticket
func (p *Parser) generateHeader(ticket types.Ticket) string {
	prefix := ""
	switch ticket.Type {
	case types.TicketTypeEpic:
		prefix = "# epic: "
	case types.TicketTypeTask:
		prefix = "## task: "
	case types.TicketTypeSubtask:
		prefix = "### subtask: "
	}

	title := ticket.Title
	if ticket.Key != "" {
		// Remove key from title if it's already there
		title = p.removeJiraKey(title)
		title = fmt.Sprintf("%s [%s]", title, ticket.Key)
	}

	return prefix + title
}

// removeJiraKey removes a Jira key from text
func (p *Parser) removeJiraKey(text string) string {
	re := regexp.MustCompile(`\s*\[?[A-Z]+-\d+\]?\s*`)
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}

// GetEpicFilePath returns the file path for an epic
func (p *Parser) GetEpicFilePath(epicKey string) string {
	if epicKey == "" {
		return filepath.Join(p.dataDir, "tickets", "inbox.md")
	}
	return filepath.Join(p.dataDir, "tickets", fmt.Sprintf("%s.md", epicKey))
}

// GetInboxFilePath returns the path to the inbox file
func (p *Parser) GetInboxFilePath() string {
	return filepath.Join(p.dataDir, "tickets", "inbox.md")
}

// EnsureFileExists ensures a file exists with basic structure
func (p *Parser) EnsureFileExists(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create directory if needed
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Create empty file
		return os.WriteFile(filePath, []byte(""), 0644)
	}
	return nil
}
