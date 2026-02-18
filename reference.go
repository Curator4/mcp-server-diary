package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- types ---

type MealReference struct {
	Name     string `json:"name"`
	Calories int    `json:"calories"`
	Protein  int    `json:"protein"`
	Notes    string `json:"notes,omitempty"`
}

type AddMealReferenceInput struct {
	Name     string `json:"name" jsonschema:"Meal name (e.g. overnight oats)"`
	Calories int    `json:"calories" jsonschema:"Calorie count"`
	Protein  int    `json:"protein" jsonschema:"Protein in grams"`
	Notes    string `json:"notes,omitempty" jsonschema:"Optional notes about the meal"`
}

type AddMealReferenceOutput struct {
	Name       string `json:"name" jsonschema:"The meal that was added/updated"`
	TotalMeals int    `json:"totalMeals" jsonschema:"Total number of meals in the reference"`
}

type GetMealReferenceInput struct {
	Name string `json:"name,omitempty" jsonschema:"Search term (optional — omit for all meals, case-insensitive substring match)"`
}

type GetMealReferenceOutput struct {
	Meals []MealReference `json:"meals"`
	Count int             `json:"count"`
}

// --- helpers ---

func mealReferencePath() string {
	return filepath.Join(themisPath, "meals_reference.json")
}

func loadMealReferences() ([]MealReference, error) {
	data, err := os.ReadFile(mealReferencePath())
	if err != nil {
		if os.IsNotExist(err) {
			return []MealReference{}, nil
		}
		return nil, fmt.Errorf("failed to read meal reference: %w", err)
	}

	var meals []MealReference
	if err := json.Unmarshal(data, &meals); err != nil {
		return nil, fmt.Errorf("failed to parse meal reference: %w", err)
	}

	return meals, nil
}

func saveMealReferences(meals []MealReference) error {
	data, err := json.MarshalIndent(meals, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meal reference: %w", err)
	}

	return os.WriteFile(mealReferencePath(), data, 0644)
}

// --- handlers ---

func handleAddMealReference(ctx context.Context, req *mcp.CallToolRequest, input AddMealReferenceInput) (
	*mcp.CallToolResult,
	AddMealReferenceOutput,
	error,
) {
	meals, err := loadMealReferences()
	if err != nil {
		return nil, AddMealReferenceOutput{}, err
	}

	// check if meal already exists (case-insensitive)
	found := false
	for i, m := range meals {
		if strings.EqualFold(m.Name, input.Name) {
			meals[i].Calories = input.Calories
			meals[i].Protein = input.Protein
			meals[i].Notes = input.Notes
			found = true
			break
		}
	}

	if !found {
		meals = append(meals, MealReference{
			Name:     input.Name,
			Calories: input.Calories,
			Protein:  input.Protein,
			Notes:    input.Notes,
		})
	}

	if err := saveMealReferences(meals); err != nil {
		return nil, AddMealReferenceOutput{}, err
	}

	return nil, AddMealReferenceOutput{
		Name:       input.Name,
		TotalMeals: len(meals),
	}, nil
}

func handleGetMealReference(ctx context.Context, req *mcp.CallToolRequest, input GetMealReferenceInput) (
	*mcp.CallToolResult,
	GetMealReferenceOutput,
	error,
) {
	meals, err := loadMealReferences()
	if err != nil {
		return nil, GetMealReferenceOutput{}, err
	}

	// if no search term, return all
	if input.Name == "" {
		return nil, GetMealReferenceOutput{Meals: meals, Count: len(meals)}, nil
	}

	// case-insensitive substring match
	search := strings.ToLower(input.Name)
	var matched []MealReference
	for _, m := range meals {
		if strings.Contains(strings.ToLower(m.Name), search) {
			matched = append(matched, m)
		}
	}

	return nil, GetMealReferenceOutput{Meals: matched, Count: len(matched)}, nil
}
