package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func getVaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "obsidian-vault", "themis")
}

var themisPath = getVaultPath()

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "themis", Version: "V1.1.0"}, nil)

	// diary
	mcp.AddTool(server, &mcp.Tool{Name: "getRecentEntries", Description: "fetches diary entries from the latest N number of days"}, handleGetRecentEntries)

	// nutrition
	mcp.AddTool(server, &mcp.Tool{Name: "logMeal", Description: "log a meal to today's diary note under ## nutrition"}, handleLogMeal)
	mcp.AddTool(server, &mcp.Tool{Name: "getNutritionSummary", Description: "get nutrition entries and macro totals for a date range"}, handleGetNutritionSummary)

	// workout
	mcp.AddTool(server, &mcp.Tool{Name: "logWorkout", Description: "log a workout to today's diary note under ## workout"}, handleLogWorkout)
	mcp.AddTool(server, &mcp.Tool{Name: "getWorkoutSummary", Description: "get workout entries for a date range"}, handleGetWorkoutSummary)

	// meal reference
	mcp.AddTool(server, &mcp.Tool{Name: "addMealReference", Description: "add or update a meal in the reference database with macros"}, handleAddMealReference)
	mcp.AddTool(server, &mcp.Tool{Name: "getMealReference", Description: "look up meals from the reference database (substring search or list all)"}, handleGetMealReference)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
