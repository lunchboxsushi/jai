package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lunchboxsushi/jai/internal/markdown"
	"github.com/lunchboxsushi/jai/internal/types"
)

func isMarkdownFile(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown")
}

func findAllTickets(dataDir string, parser *markdown.Parser) ([]types.Ticket, error) {
	ticketsDir := filepath.Join(dataDir, "tickets")
	files, err := os.ReadDir(ticketsDir)
	if err != nil {
		return nil, fmt.Errorf("could not read tickets directory: %w", err)
	}

	var allTickets []types.Ticket
	for _, file := range files {
		if file.IsDir() || !isMarkdownFile(file.Name()) {
			continue
		}
		filePath := filepath.Join(ticketsDir, file.Name())
		mdFile, err := parser.ParseFile(filePath)
		if err != nil {
			// Log or handle error if a file can't be parsed
			continue
		}
		allTickets = append(allTickets, mdFile.Tickets...)
	}

	return allTickets, nil
}
