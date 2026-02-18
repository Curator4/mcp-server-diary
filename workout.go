package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- input/output types ---

type LogWorkoutInput struct {
	Description string `json:"description" jsonschema:"What you did (e.g. chest day: bench 80kg 4x8, incline DB 30kg 3x10)"`
}

type LogWorkoutOutput struct {
	Date  string `json:"date" jsonschema:"Date the workout was logged"`
	Entry string `json:"entry" jsonschema:"The formatted entry that was appended"`
	Path  string `json:"path" jsonschema:"Path to the daily note"`
}

type GetWorkoutSummaryInput struct {
	Date string `json:"date,omitempty" jsonschema:"Specific date in YYYY-MM-DD format (optional)"`
	Days int    `json:"days,omitempty" jsonschema:"Number of recent days to summarize (optional, overrides date)"`
}

type WorkoutDay struct {
	Date    string   `json:"date"`
	Entries []string `json:"entries"`
	Count   int      `json:"count"`
}

type WorkoutSummaryOutput struct {
	Days       []WorkoutDay `json:"days"`
	TotalCount int          `json:"totalCount"`
	DayCount   int          `json:"dayCount"`
}

// --- handlers ---

func handleLogWorkout(ctx context.Context, req *mcp.CallToolRequest, input LogWorkoutInput) (
	*mcp.CallToolResult,
	LogWorkoutOutput,
	error,
) {
	now := time.Now()
	path, err := ensureDailyNote(now)
	if err != nil {
		return nil, LogWorkoutOutput{}, fmt.Errorf("failed to ensure daily note: %w", err)
	}

	entry := fmt.Sprintf("- %s | %s", now.Format("15:04"), input.Description)

	if err := appendToSection(path, "workout", entry); err != nil {
		return nil, LogWorkoutOutput{}, fmt.Errorf("failed to append workout: %w", err)
	}

	return nil, LogWorkoutOutput{
		Date:  now.Format("2006-01-02"),
		Entry: entry,
		Path:  path,
	}, nil
}

func handleGetWorkoutSummary(ctx context.Context, req *mcp.CallToolRequest, input GetWorkoutSummaryInput) (
	*mcp.CallToolResult,
	WorkoutSummaryOutput,
	error,
) {
	dates, err := resolveDateRange(input.Date, input.Days)
	if err != nil {
		return nil, WorkoutSummaryOutput{}, err
	}

	var days []WorkoutDay
	totalCount := 0

	for _, date := range dates {
		path := dateToFilePath(date)
		section, err := readSection(path, "workout")
		if err != nil || section == "" {
			continue
		}

		entries := parseWorkoutEntries(section)
		if len(entries) == 0 {
			continue
		}

		totalCount += len(entries)
		days = append(days, WorkoutDay{
			Date:    date.Format("2006-01-02"),
			Entries: entries,
			Count:   len(entries),
		})
	}

	return nil, WorkoutSummaryOutput{
		Days:       days,
		TotalCount: totalCount,
		DayCount:   len(days),
	}, nil
}

// parseWorkoutEntries splits the section content into individual entries.
// Each line starting with "- " is an entry.
func parseWorkoutEntries(section string) []string {
	var entries []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			entries = append(entries, strings.TrimPrefix(line, "- "))
		}
	}
	return entries
}
