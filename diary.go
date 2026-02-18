package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- types (relocated from main.go) ---

type Entry struct {
	Date     string `json:"date" jsonschema:"Entry date in YYYY-MM-DD format"`
	FilePath string `json:"path" jsonschema:"Full path to the diary entry file"`
	Content  string `json:"content" jsonschema:"Full markdown content of the entry"`
}

type GetRecentEntriesInput struct {
	Days int `json:"days" jsonschema:"Number of days to retrieve (e.g., 7 for last week)"`
}

type EntriesOutput struct {
	Entries []Entry `json:"entries" jsonschema:"List of diary entries, sorted newest first"`
	Count   int     `json:"count" jsonschema:"Total number of entries returned"`
}

// --- handler (relocated from main.go) ---

func handleGetRecentEntries(ctx context.Context, req *mcp.CallToolRequest, input GetRecentEntriesInput) (
	*mcp.CallToolResult,
	EntriesOutput,
	error,
) {
	cutoff := time.Now().AddDate(0, 0, -input.Days)
	entries, err := getEntries(func(date time.Time) bool {
		return date.After(cutoff) || date.Equal(cutoff)
	})
	if err != nil {
		return nil, EntriesOutput{}, fmt.Errorf("failed to get entries: %w", err)
	}

	return nil, EntriesOutput{Entries: entries, Count: len(entries)}, nil
}

// --- helpers (relocated from main.go) ---

func getEntries(filter func(date time.Time) bool) ([]Entry, error) {
	var entries []Entry

	err := filepath.WalkDir(themisPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("error accessing %s: %v", path, err)
			return nil
		}

		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		dateStr := strings.TrimSuffix(d.Name(), ".md")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil
		}

		if !filter(date) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("error reading %s: %v", path, err)
			return nil
		}

		entries = append(entries, Entry{
			Date:     dateStr,
			FilePath: path,
			Content:  string(content),
		})

		return nil
	})

	return entries, err
}

// --- new shared helpers ---

// dateToFilePath builds the full path for a daily note: themisPath/YYYY/YYYY-MM/YYYY-MM-DD.md
func dateToFilePath(date time.Time) string {
	y := date.Format("2006")
	m := date.Format("2006-01")
	d := date.Format("2006-01-02")
	return filepath.Join(themisPath, y, m, d+".md")
}

// ensureDailyNote creates the daily note + directories if missing, using the standard template.
// Returns the file path.
func ensureDailyNote(date time.Time) (string, error) {
	path := dateToFilePath(date)

	// if it already exists, just return the path
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// create parent dirs
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// write template
	y := date.Format("2006")
	d := date.Format("2006-01-02")
	weekday := date.Format("Monday")
	_, week := date.ISOWeek()
	template := fmt.Sprintf(`# entry: %s

Tags: #themis #%s
%s · Week %02d · [[reference]]

## log

## nutrition

## workout

## stats
`, d, y, weekday, week)

	if err := os.WriteFile(path, []byte(template), 0644); err != nil {
		return "", fmt.Errorf("failed to create daily note: %w", err)
	}

	return path, nil
}

// appendToSection finds ## sectionName in the file and appends content after the last line
// in that section (before the next ## or EOF). If the section doesn't exist, appends it at EOF.
func appendToSection(filePath, sectionName, content string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	header := "## " + sectionName

	// find the section
	sectionStart := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			sectionStart = i
			break
		}
	}

	var result []string

	if sectionStart == -1 {
		// section doesn't exist — append at end of file
		// trim trailing empty lines, add section, then content
		result = append(lines, "", header, "", content, "")
	} else {
		// find the end of this section (next ## or EOF)
		insertAt := len(lines)
		for i := sectionStart + 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "## ") {
				insertAt = i
				break
			}
		}

		// insert content before the next section (or EOF)
		// we want to put it after any existing content in the section
		// back up past trailing blank lines within the section
		contentEnd := insertAt
		for contentEnd > sectionStart+1 && strings.TrimSpace(lines[contentEnd-1]) == "" {
			contentEnd--
		}

		// build result: everything up to contentEnd, new content, blank line, rest
		result = append(result, lines[:contentEnd]...)
		result = append(result, content)
		result = append(result, "")
		if insertAt < len(lines) {
			result = append(result, lines[insertAt:]...)
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(result, "\n")), 0644)
}

// readSection extracts the content between ## sectionName and the next ## or EOF.
// Returns empty string if section not found.
func readSection(filePath, sectionName string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	header := "## " + sectionName

	sectionStart := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			sectionStart = i
			break
		}
	}

	if sectionStart == -1 {
		return "", nil
	}

	// collect lines until next ## or EOF
	var sectionLines []string
	for i := sectionStart + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			break
		}
		sectionLines = append(sectionLines, lines[i])
	}

	return strings.TrimSpace(strings.Join(sectionLines, "\n")), nil
}

// resolveDateRange turns flexible input into a slice of dates.
// - days > 0: last N days from today (inclusive)
// - dateStr set: single specific date
// - neither: just today
func resolveDateRange(dateStr string, days int) ([]time.Time, error) {
	if days > 0 {
		var dates []time.Time
		today := time.Now()
		for i := 0; i < days; i++ {
			dates = append(dates, today.AddDate(0, 0, -i))
		}
		return dates, nil
	}

	if dateStr != "" {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date format (expected YYYY-MM-DD): %w", err)
		}
		return []time.Time{date}, nil
	}

	return []time.Time{time.Now()}, nil
}
