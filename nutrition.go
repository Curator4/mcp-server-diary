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

type LogMealInput struct {
	Description string `json:"description" jsonschema:"What you ate (e.g. overnight oats with banana)"`
	Calories    int    `json:"calories" jsonschema:"Calorie count"`
	Protein     int    `json:"protein" jsonschema:"Protein in grams"`
}

type LogMealOutput struct {
	Date  string `json:"date" jsonschema:"Date the meal was logged"`
	Entry string `json:"entry" jsonschema:"The formatted entry that was appended"`
	Path  string `json:"path" jsonschema:"Path to the daily note"`
}

type GetNutritionSummaryInput struct {
	Date string `json:"date,omitempty" jsonschema:"Specific date in YYYY-MM-DD format (optional)"`
	Days int    `json:"days,omitempty" jsonschema:"Number of recent days to summarize (optional, overrides date)"`
}

type NutritionEntry struct {
	Time        string `json:"time"`
	Description string `json:"description"`
	Calories    int    `json:"calories"`
	Protein     int    `json:"protein"`
}

type NutritionDay struct {
	Date          string           `json:"date"`
	Entries       []NutritionEntry `json:"entries"`
	TotalCalories int              `json:"totalCalories"`
	TotalProtein  int              `json:"totalProtein"`
}

type NutritionSummaryOutput struct {
	Days          []NutritionDay `json:"days"`
	TotalCalories int            `json:"totalCalories"`
	TotalProtein  int            `json:"totalProtein"`
	DayCount      int            `json:"dayCount"`
}

// --- handlers ---

func handleLogMeal(ctx context.Context, req *mcp.CallToolRequest, input LogMealInput) (
	*mcp.CallToolResult,
	LogMealOutput,
	error,
) {
	now := time.Now()
	path, err := ensureDailyNote(now)
	if err != nil {
		return nil, LogMealOutput{}, fmt.Errorf("failed to ensure daily note: %w", err)
	}

	entry := fmt.Sprintf("- %s | %s | %d kcal | %dg protein", now.Format("15:04"), input.Description, input.Calories, input.Protein)

	if err := appendToSection(path, "nutrition", entry); err != nil {
		return nil, LogMealOutput{}, fmt.Errorf("failed to append meal: %w", err)
	}

	return nil, LogMealOutput{
		Date:  now.Format("2006-01-02"),
		Entry: entry,
		Path:  path,
	}, nil
}

func handleGetNutritionSummary(ctx context.Context, req *mcp.CallToolRequest, input GetNutritionSummaryInput) (
	*mcp.CallToolResult,
	NutritionSummaryOutput,
	error,
) {
	dates, err := resolveDateRange(input.Date, input.Days)
	if err != nil {
		return nil, NutritionSummaryOutput{}, err
	}

	var days []NutritionDay
	grandTotalCal := 0
	grandTotalPro := 0

	for _, date := range dates {
		path := dateToFilePath(date)
		section, err := readSection(path, "nutrition")
		if err != nil || section == "" {
			continue
		}

		entries := parseNutritionEntries(section)
		if len(entries) == 0 {
			continue
		}

		dayCal := 0
		dayPro := 0
		for _, e := range entries {
			dayCal += e.Calories
			dayPro += e.Protein
		}

		grandTotalCal += dayCal
		grandTotalPro += dayPro

		days = append(days, NutritionDay{
			Date:          date.Format("2006-01-02"),
			Entries:       entries,
			TotalCalories: dayCal,
			TotalProtein:  dayPro,
		})
	}

	return nil, NutritionSummaryOutput{
		Days:          days,
		TotalCalories: grandTotalCal,
		TotalProtein:  grandTotalPro,
		DayCount:      len(days),
	}, nil
}

// parseNutritionEntries parses lines like "- 09:15 | overnight oats | 450 kcal | 15g protein"
func parseNutritionEntries(section string) []NutritionEntry {
	var entries []NutritionEntry

	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		line = strings.TrimPrefix(line, "- ")

		parts := strings.Split(line, " | ")
		if len(parts) < 4 {
			continue
		}

		cal, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(parts[2]), " kcal"))
		pro, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(parts[3]), "g protein"))

		entries = append(entries, NutritionEntry{
			Time:        strings.TrimSpace(parts[0]),
			Description: strings.TrimSpace(parts[1]),
			Calories:    cal,
			Protein:     pro,
		})
	}

	return entries
}
