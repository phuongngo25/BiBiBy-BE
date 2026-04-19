package spoonacular

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const baseURL = "https://api.spoonacular.com"

// Client is the HTTP client for the Spoonacular API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Spoonacular client. Returns nil if no API key is set,
// so callers must guard against a nil client.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// ---------------------------------------------------------------------------
// DTOs — Spoonacular API response shapes
// ---------------------------------------------------------------------------

// NutrientInfo is a single macro/micro value returned inside a recipe result.
type NutrientInfo struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

// NutritionSummary wraps a list of nutrients for one recipe.
type NutritionSummary struct {
	Nutrients []NutrientInfo `json:"nutrients"`
}

// GetNutrient finds a specific nutrient by name (e.g., "Calories", "Protein").
func (n NutritionSummary) GetNutrient(name string) float64 {
	for _, nut := range n.Nutrients {
		if nut.Name == name {
			return nut.Amount
		}
	}
	return 0
}

// RecipeResult is one item inside a ComplexSearch response.
type RecipeResult struct {
	ID          int              `json:"id"`
	Title       string           `json:"title"`
	Image       string           `json:"image"`
	Servings    float64          `json:"servings"`
	Vegan       bool             `json:"vegan"`
	Vegetarian  bool             `json:"vegetarian"`
	GlutenFree  bool             `json:"glutenFree"`
	DairyFree   bool             `json:"dairyFree"`
	Nutrition   NutritionSummary `json:"nutrition"`
}

type complexSearchResponse struct {
	Results []RecipeResult `json:"results"`
	Total   int            `json:"totalResults"`
}

// NutrientSearchResult is one item inside a FindByNutrients response.
type NutrientSearchResult struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Image    string  `json:"image"`
	Calories float64 `json:"calories"`
	Protein  string  `json:"protein"`
	Fat      string  `json:"fat"`
	Carbs    string  `json:"carbs"`
}

// IngredientSearchResult is one item inside a FindByIngredients response.
type IngredientSearchResult struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
}

// ---------------------------------------------------------------------------
// API Methods
// ---------------------------------------------------------------------------

// ComplexSearch calls the /recipes/complexSearch endpoint.
// diet and intolerances can be empty strings (not sent if empty).
// maxCarbs of 0 means no carb ceiling is applied.
func (c *Client) ComplexSearch(ctx context.Context, query, diet, intolerances string, maxCarbs int) ([]RecipeResult, error) {
	params := url.Values{}
	params.Set("apiKey", c.apiKey)
	params.Set("query", query)
	params.Set("addRecipeNutrition", "true")
	params.Set("number", "10")
	if diet != "" {
		params.Set("diet", diet)
	}
	if intolerances != "" {
		params.Set("intolerances", intolerances)
	}
	if maxCarbs > 0 {
		params.Set("maxCarbs", strconv.Itoa(maxCarbs))
	}

	endpoint := fmt.Sprintf("%s/recipes/complexSearch?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spoonacular: unexpected status %d", resp.StatusCode)
	}

	var result complexSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("spoonacular: decode response: %w", err)
	}
	return result.Results, nil
}

// FindByNutrients calls the /recipes/findByNutrients endpoint.
// Zero values are treated as "unconstrained" and are not sent as params.
func (c *Client) FindByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]NutrientSearchResult, error) {
	params := url.Values{}
	params.Set("apiKey", c.apiKey)
	params.Set("number", "10")
	if minProtein > 0 {
		params.Set("minProtein", fmt.Sprintf("%.0f", minProtein))
	}
	if maxFat > 0 {
		params.Set("maxFat", fmt.Sprintf("%.0f", maxFat))
	}
	if minCalories > 0 {
		params.Set("minCalories", fmt.Sprintf("%.0f", minCalories))
	}
	if maxCalories > 0 {
		params.Set("maxCalories", fmt.Sprintf("%.0f", maxCalories))
	}

	endpoint := fmt.Sprintf("%s/recipes/findByNutrients?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spoonacular: unexpected status %d", resp.StatusCode)
	}

	var results []NutrientSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("spoonacular: decode response: %w", err)
	}
	return results, nil
}

// FindByIngredients calls the /recipes/findByIngredients endpoint.
func (c *Client) FindByIngredients(ctx context.Context, ingredients string) ([]IngredientSearchResult, error) {
	params := url.Values{}
	params.Set("apiKey", c.apiKey)
	params.Set("ingredients", ingredients)
	params.Set("number", "10")

	endpoint := fmt.Sprintf("%s/recipes/findByIngredients?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spoonacular: unexpected status %d", resp.StatusCode)
	}

	var results []IngredientSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("spoonacular: decode response: %w", err)
	}
	return results, nil
}
