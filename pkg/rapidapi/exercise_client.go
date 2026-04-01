package rapidapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"nutrix-backend/internal/domain"
)

const (
	ascendAPIHost  = "edb-with-gifs-and-images-by-ascendapi.p.rapidapi.com"
	ascendBaseURL  = "https://" + ascendAPIHost + "/api/v1"
	heatmapBaseURL = "https://muscle-visualizer-api.p.rapidapi.com/api/v1/visualize"
)

// ExerciseClient proxies the "EDB with GIFs and Images by AscendAPI" endpoints.
type ExerciseClient struct {
	httpClient *http.Client
	apiKey     string
}

func NewExerciseClient(apiKey string) *ExerciseClient {
	return &ExerciseClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiKey:     apiKey,
	}
}

// ─── AscendAPI Inner Response Mapping ─────────────────────────────────────────

type ascendListExercise struct {
	ID            string   `json:"exerciseId"`
	Name          string   `json:"name"`
	ImageUrl      string   `json:"imageUrl"`
	BodyParts     []string `json:"bodyParts,omitempty"`
	TargetMuscles []string `json:"targetMuscles,omitempty"`
}

type ascendListResponse struct {
	Data []ascendListExercise `json:"data"`
}

type ascendDetailGif struct {
	P720  string `json:"720p"`
	P1080 string `json:"1080p"`
}

type ascendDetailExercise struct {
	ID            string          `json:"exerciseId"`
	Name          string          `json:"name"`
	Gifs          ascendDetailGif `json:"gifUrls"`
	Instructions  []string        `json:"instructions"`
	Equipments    []string        `json:"equipments"`
	TargetMuscles []string        `json:"targetMuscles"`
}

type ascendDetailResponse struct {
	Success bool                 `json:"success"`
	Data    ascendDetailExercise `json:"data"`
}

// ─── Public Proxy Methodology ─────────────────────────────────────────────────

// FetchByBodyParts calls /api/v1/exercises/bodyparts?bodyParts={parts}&limit=30
func (c *ExerciseClient) FetchByBodyParts(parts string) ([]domain.ExerciseListItem, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("rapidapi: RAPIDAPI_KEY is not set in environment")
	}

	reqURL := fmt.Sprintf("%s/exercises/bodyparts?bodyParts=%s&limit=30", ascendBaseURL, url.QueryEscape(parts))

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("rapidapi: failed to build request: %w", err)
	}
	c.setHeaders(req)

	log.Printf("DEBUG - Calling URL: %s", reqURL)
	log.Printf("DEBUG - Using API Key Length: %d", len(c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rapidapi: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rapidapi: returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed ascendListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("rapidapi: failed to decode response: %w", err)
	}

	mapped := make([]domain.ExerciseListItem, 0, len(parsed.Data))
	for _, a := range parsed.Data {
		mapped = append(mapped, domain.ExerciseListItem{
			ExerciseID:    a.ID,
			Name:          a.Name,
			ImageUrl:      a.ImageUrl,
			BodyParts:     a.BodyParts,
			TargetMuscles: a.TargetMuscles,
		})
	}
	return mapped, nil
}

// FetchExerciseByID calls /api/v1/exercises/{id}
func (c *ExerciseClient) FetchExerciseByID(id string) (*domain.ExerciseDetail, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("rapidapi: RAPIDAPI_KEY is not set in environment")
	}

	// FIX: Added the 's' perfectly here
	reqURL := fmt.Sprintf("%s/exercises/%s", ascendBaseURL, url.QueryEscape(id))

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("rapidapi: failed to build request: %w", err)
	}
	c.setHeaders(req)

	log.Printf("DEBUG - Calling URL: %s", reqURL)
	log.Printf("DEBUG - Using API Key Length: %d", len(c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rapidapi: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rapidapi: returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed ascendDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("rapidapi: failed to decode response: %w", err)
	}

	if !parsed.Success || parsed.Data.ID == "" {
		return nil, fmt.Errorf("rapidapi: exercise not found or request failed")
	}

	a := parsed.Data
	var primaryMuscle string
	if len(a.TargetMuscles) > 0 {
		primaryMuscle = a.TargetMuscles[0]
	}

	return &domain.ExerciseDetail{
		ExerciseID: a.ID,
		Name:       a.Name,
		GifUrls: domain.GifUrls{
			P720:  a.Gifs.P720,
			P1080: a.Gifs.P1080,
		},
		Instructions:     a.Instructions,
		Equipments:       a.Equipments,
		MuscleHeatmapUrl: buildHeatmapURL(primaryMuscle),
	}, nil
}

// ─── Private Helpers ──────────────────────────────────────────────────────────

func (c *ExerciseClient) setHeaders(req *http.Request) {
	req.Header.Add("x-rapidapi-key", c.apiKey)
	req.Header.Add("x-rapidapi-host", ascendAPIHost)
	req.Header.Add("Content-Type", "application/json")
}

func buildHeatmapURL(muscle string) string {
	if muscle == "" {
		return ""
	}
	params := url.Values{}
	params.Set("muscles", muscle)
	params.Set("color", "#D20A2E")
	params.Set("gender", "male")
	params.Set("background", "transparent")
	params.Set("size", "small")
	params.Set("format", "jpeg")
	return heatmapBaseURL + "?" + params.Encode()
}
