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

	content := p.GenerateMarkdown(tickets)
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
	inMetadata := false

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
			inMetadata = false
			lines = []string{}
			continue
		}

		if inTicket {
			// Check for metadata section start
			if strings.TrimSpace(line) == "---" {
				// Look ahead for metadata marker
				if scanner.Scan() {
					lineNum++
					nextLine := strings.TrimSpace(scanner.Text())
					if nextLine == "*Metadata:*" {
						inMetadata = true
						// Parse existing metadata lines before the marker
						p.parseMetadataLines(lines, currentTicket)
						lines = []string{}
						continue
					} else {
						// Not metadata, add both lines back
						lines = append(lines, line)
						lines = append(lines, scanner.Text())
					}
				} else {
					// End of file, add the line
					lines = append(lines, line)
				}
			} else if inMetadata {
				// Check for metadata section end
				if strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "" {
					inMetadata = false
					continue
				}
				// Parse metadata line
				p.parseMetadataLine(line, currentTicket)
			} else {
				lines = append(lines, line)
			}
		}
	}

	// Don't forget the last ticket
	if inTicket {
		// Parse any remaining metadata lines
		if !inMetadata {
			p.parseMetadataLines(lines, currentTicket)
		}
		currentTicket.RawContent = strings.TrimSpace(strings.Join(lines, "\n"))
		tickets = append(tickets, *currentTicket)
	}

	return tickets
}

// parseMetadataLines parses metadata lines for a ticket
func (p *Parser) parseMetadataLines(lines []string, ticket *types.Ticket) {
	for _, metaLine := range lines {
		metaLine = strings.TrimSpace(metaLine)
		p.parseMetadataLine(metaLine, ticket)
	}
}

// parseMetadataLine parses a single metadata line
func (p *Parser) parseMetadataLine(metaLine string, ticket *types.Ticket) {
	metaLine = strings.TrimSpace(metaLine)
	if !strings.HasPrefix(metaLine, "- ") {
		return
	}

	// Remove the "- " prefix
	metaLine = strings.TrimPrefix(metaLine, "- ")

	// Parse different metadata fields
	switch {
	case strings.HasPrefix(metaLine, "Key:"):
		ticket.Key = strings.TrimSpace(strings.TrimPrefix(metaLine, "Key:"))
	case strings.HasPrefix(metaLine, "Status:"):
		ticket.Status = strings.TrimSpace(strings.TrimPrefix(metaLine, "Status:"))
	case strings.HasPrefix(metaLine, "Priority:"):
		ticket.Priority = strings.TrimSpace(strings.TrimPrefix(metaLine, "Priority:"))
	case strings.HasPrefix(metaLine, "EpicKey:"):
		ticket.EpicKey = strings.TrimSpace(strings.TrimPrefix(metaLine, "EpicKey:"))
	case strings.HasPrefix(metaLine, "ParentKey:"):
		// For tasks, ParentKey refers to the epic
		if ticket.Type == types.TicketTypeTask {
			ticket.EpicKey = strings.TrimSpace(strings.TrimPrefix(metaLine, "ParentKey:"))
		} else if ticket.Type == types.TicketTypeSubtask {
			// For subtasks, ParentKey refers to the parent task
			ticket.ParentKey = strings.TrimSpace(strings.TrimPrefix(metaLine, "ParentKey:"))
		}
	case strings.HasPrefix(metaLine, "TaskKey:"):
		// For subtasks, TaskKey refers to the parent task
		if ticket.Type == types.TicketTypeSubtask {
			ticket.ParentKey = strings.TrimSpace(strings.TrimPrefix(metaLine, "TaskKey:"))
		}
	case strings.HasPrefix(metaLine, "ParentTask:"):
		ticket.ParentKey = strings.TrimSpace(strings.TrimPrefix(metaLine, "ParentTask:"))
	case strings.HasPrefix(metaLine, "ParentEpic:"):
		ticket.EpicKey = strings.TrimSpace(strings.TrimPrefix(metaLine, "ParentEpic:"))
	}
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

// GenerateMarkdown generates markdown content from tickets
func (p *Parser) GenerateMarkdown(tickets []types.Ticket) string {
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

		// Add metadata section
		metaLines := []string{"---", "*Metadata:*"}
		if ticket.Key != "" {
			metaLines = append(metaLines, fmt.Sprintf("- Key: %s", ticket.Key))
		}
		if ticket.Status != "" {
			metaLines = append(metaLines, fmt.Sprintf("- Status: %s", ticket.Status))
		}
		if ticket.Priority != "" {
			metaLines = append(metaLines, fmt.Sprintf("- Priority: %s", ticket.Priority))
		}

		// Add appropriate parent references based on ticket type
		switch ticket.Type {
		case types.TicketTypeEpic:
			// Epics don't have parents, but may have EpicKey for consistency
			if ticket.EpicKey != "" {
				metaLines = append(metaLines, fmt.Sprintf("- EpicKey: %s", ticket.EpicKey))
			}
		case types.TicketTypeTask:
			// Tasks have ParentKey (epic)
			if ticket.EpicKey != "" {
				metaLines = append(metaLines, fmt.Sprintf("- ParentKey: %s", ticket.EpicKey))
			}
		case types.TicketTypeSubtask:
			// Subtasks have TaskKey (parent task)
			if ticket.ParentKey != "" {
				metaLines = append(metaLines, fmt.Sprintf("- TaskKey: %s", ticket.ParentKey))
			}
		}

		metaLines = append(metaLines, "")
		lines = append(lines, metaLines...)

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
		title = p.RemoveJiraKey(title)
		title = fmt.Sprintf("%s [%s]", title, ticket.Key)
	}

	return prefix + title
}

// RemoveJiraKey removes a Jira key from text
func (p *Parser) RemoveJiraKey(text string) string {
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

// GetTaskFilePath returns the file path for a task (when no epic context)
func (p *Parser) GetTaskFilePath(taskKey string) string {
	if taskKey == "" {
		return filepath.Join(p.dataDir, "tickets", "inbox.md")
	}
	return filepath.Join(p.dataDir, "tickets", fmt.Sprintf("%s.md", taskKey))
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
