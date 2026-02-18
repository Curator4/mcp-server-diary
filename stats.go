package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- input/output types ---

type LogWeightInput struct {
	Kg float64 `json:"kg" jsonschema:"Weight in kilograms"`
}

type LogWeightOutput struct {
	Date  string `json:"date" jsonschema:"Date the weight was logged"`
	Entry string `json:"entry" jsonschema:"The formatted entry that was appended"`
	Path  string `json:"path" jsonschema:"Path to the daily note"`
}

type GetWeightSummaryInput struct {
	Date string `json:"date,omitempty" jsonschema:"Specific date in YYYY-MM-DD format (optional)"`
	Days int    `json:"days,omitempty" jsonschema:"Number of recent days to summarize (optional, overrides date)"`
}

type WeightEntry struct {
	Date string  `json:"date"`
	Kg   float64 `json:"kg"`
}

type WeightSummaryOutput struct {
	Entries []WeightEntry `json:"entries"`
	Count   int           `json:"count"`
	AvgKg   float64       `json:"avgKg,omitempty"`
	MinKg   float64       `json:"minKg,omitempty"`
	MaxKg   float64       `json:"maxKg,omitempty"`
	DeltaKg float64       `json:"deltaKg,omitempty"`
}

// --- handlers ---

func handleLogWeight(ctx context.Context, req *mcp.CallToolRequest, input LogWeightInput) (
	*mcp.CallToolResult,
	LogWeightOutput,
	error,
) {
	now := time.Now()
	path, err := ensureDailyNote(now)
	if err != nil {
		return nil, LogWeightOutput{}, fmt.Errorf("failed to ensure daily note: %w", err)
	}

	entry := fmt.Sprintf("- %skg", formatWeight(input.Kg))

	if err := appendToSection(path, "stats", entry); err != nil {
		return nil, LogWeightOutput{}, fmt.Errorf("failed to append weight: %w", err)
	}

	return nil, LogWeightOutput{
		Date:  now.Format("2006-01-02"),
		Entry: entry,
		Path:  path,
	}, nil
}

func handleGetWeightSummary(ctx context.Context, req *mcp.CallToolRequest, input GetWeightSummaryInput) (
	*mcp.CallToolResult,
	WeightSummaryOutput,
	error,
) {
	dates, err := resolveDateRange(input.Date, input.Days)
	if err != nil {
		return nil, WeightSummaryOutput{}, err
	}

	var entries []WeightEntry

	for _, date := range dates {
		path := dateToFilePath(date)
		section, err := readSection(path, "stats")
		if err != nil || section == "" {
			continue
		}

		kg, ok := parseWeightEntry(section)
		if !ok {
			continue
		}

		entries = append(entries, WeightEntry{
			Date: date.Format("2006-01-02"),
			Kg:   kg,
		})
	}

	out := WeightSummaryOutput{
		Entries: entries,
		Count:   len(entries),
	}

	if len(entries) > 0 {
		sum := 0.0
		out.MinKg = entries[0].Kg
		out.MaxKg = entries[0].Kg

		for _, e := range entries {
			sum += e.Kg
			if e.Kg < out.MinKg {
				out.MinKg = e.Kg
			}
			if e.Kg > out.MaxKg {
				out.MaxKg = e.Kg
			}
		}

		out.AvgKg = sum / float64(len(entries))
		// delta: earliest to latest (entries are newest-first from resolveDateRange)
		out.DeltaKg = entries[0].Kg - entries[len(entries)-1].Kg
	}

	return nil, out, nil
}

// parseWeightEntry extracts the kg value from lines like "- 80kg" or "- 80.5kg".
// Returns the first match found.
func parseWeightEntry(section string) (float64, bool) {
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSuffix(strings.TrimSpace(line), "kg")

		kg, err := strconv.ParseFloat(line, 64)
		if err != nil {
			continue
		}
		return kg, true
	}
	return 0, false
}

// formatWeight formats a float nicely — no trailing zeros.
func formatWeight(kg float64) string {
	if kg == float64(int(kg)) {
		return strconv.Itoa(int(kg))
	}
	return strconv.FormatFloat(kg, 'f', 1, 64)
}
