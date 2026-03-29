package spoonacular

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is the HTTP client for the Spoonacular API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewClient returns a configured Spoonacular API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.spoonacular.com",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ---------------------------------------------------------------------------
// Intermediate DTOs — map the Spoonacular JSON schema to Go structs.
// These are NOT domain types; mapping happens in the UseCase layer.
// ---------------------------------------------------------------------------

type Nutrient struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

type NutritionInfo struct {
	Nutrients []Nutrient `json:"nutrients"`
}

type RecipeResult struct {
	ID              int           `json:"id"`
	Title           string        `json:"title"`
	Image           string        `json:"image"`
	Nutrition       NutritionInfo `json:"nutrition"`
	Vegetarian      bool          `json:"vegetarian"`
	Vegan           bool          `json:"vegan"`
	GlutenFree      bool          `json:"glutenFree"`
	DairyFree       bool          `json:"dairyFree"`
	Servings        float64       `json:"servings"`
}

type complexSearchResponse struct {
	Results []RecipeResult `json:"results"`
}

// NutrientSearchResult is the DTO returned by the findByNutrients endpoint.
type NutrientSearchResult struct {
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	Image     string  `json:"image"`
	Calories  float64 `json:"calories"`
	Protein   string  `json:"protein"`
	Fat       string  `json:"fat"`
	Carbs     string  `json:"carbs"`
}

// IngredientSearchResult is the DTO returned by the findByIngredients endpoint.
type IngredientSearchResult struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
}

// GetNutrient extracts a float nutrient value by name from NutritionInfo.
func (n *NutritionInfo) GetNutrient(name string) float64 {
	for _, nt := range n.Nutrients {
		if nt.Name == name {
			return nt.Amount
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// ComplexSearch calls /recipes/complexSearch with full nutrition included.
// ---------------------------------------------------------------------------
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

	reqURL := fmt.Sprintf("%s/recipes/complexSearch?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: complexSearch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spoonacular: complexSearch non-200 status: %d", resp.StatusCode)
	}

	var result complexSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("spoonacular: complexSearch decode: %w", err)
	}

	return result.Results, nil
}

// ---------------------------------------------------------------------------
// FindByNutrients calls /recipes/findByNutrients.
// ---------------------------------------------------------------------------
func (c *Client) FindByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]NutrientSearchResult, error) {
	params := url.Values{}
	params.Set("apiKey", c.apiKey)
	params.Set("number", "10")

	if minProtein > 0 {
		params.Set("minProtein", strconv.FormatFloat(minProtein, 'f', 1, 64))
	}
	if maxFat > 0 {
		params.Set("maxFat", strconv.FormatFloat(maxFat, 'f', 1, 64))
	}
	if minCalories > 0 {
		params.Set("minCalories", strconv.FormatFloat(minCalories, 'f', 1, 64))
	}
	if maxCalories > 0 {
		params.Set("maxCalories", strconv.FormatFloat(maxCalories, 'f', 1, 64))
	}

	reqURL := fmt.Sprintf("%s/recipes/findByNutrients?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: findByNutrients request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spoonacular: findByNutrients non-200 status: %d", resp.StatusCode)
	}

	var results []NutrientSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("spoonacular: findByNutrients decode: %w", err)
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// FindByIngredients calls /recipes/findByIngredients.
// ---------------------------------------------------------------------------
func (c *Client) FindByIngredients(ctx context.Context, ingredients string) ([]IngredientSearchResult, error) {
	params := url.Values{}
	params.Set("apiKey", c.apiKey)
	params.Set("ingredients", ingredients)
	params.Set("number", "10")

	reqURL := fmt.Sprintf("%s/recipes/findByIngredients?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spoonacular: findByIngredients request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spoonacular: findByIngredients non-200 status: %d", resp.StatusCode)
	}

	var results []IngredientSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("spoonacular: findByIngredients decode: %w", err)
	}

	return results, nil
}
