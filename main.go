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

func getVaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "obsidian-vault", "themis")
}

var themisPath = getVaultPath()

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

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "themis", Version: "V1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "getRecentEntries", Description: "fetches diary entries from the latest N number of days"}, handleGetRecentEntries)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// handlers
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

// helpers
func getEntries(filter func(date time.Time) bool) ([]Entry, error) {
	var entries []Entry

	// recursively walk through themis folder
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
