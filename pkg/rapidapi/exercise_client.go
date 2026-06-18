package rapidapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nutrix-backend/internal/domain"
)

const (
	ascendAPIHost  = "edb-with-gifs-and-images-by-ascendapi.p.rapidapi.com"
	ascendBaseURL  = "https://" + ascendAPIHost + "/api/v1"
	heatmapAPIHost = "muscle-visualizer-api.p.rapidapi.com"
	heatmapBaseURL = "https://" + heatmapAPIHost + "/api/v1/visualize"
	// assetCDNHost is the public CDN that serves exercise images + GIFs. It does
	// NOT send CORS headers, so Flutter web (CanvasKit) can't load these assets
	// directly — we proxy them through the backend (same-origin) instead.
	assetCDNHost = "assets.exercisedb.dev"
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

	// Translate the app's tab name into AscendAPI's body-part vocabulary. The API
	// accepts comma-separated values, so tabs that span several AscendAPI body
	// parts (Legs, Arms) are expanded in a single request.
	mappedParts := mapBodyPartsForAscend(parts)
	reqURL := fmt.Sprintf("%s/exercises/bodyparts?bodyParts=%s&limit=30", ascendBaseURL, url.QueryEscape(mappedParts))

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
			ExerciseID: a.ID,
			Name:       a.Name,
			// Rewrite to our backend proxy path so the browser loads the image
			// same-origin (the CDN has no CORS headers — see assetCDNHost).
			ImageUrl:      buildAssetProxyPath(a.ImageUrl),
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
			// Proxy the GIFs through the backend (same-origin) so Flutter web
			// can load them despite the CDN missing CORS headers.
			P720:  buildAssetProxyPath(a.Gifs.P720),
			P1080: buildAssetProxyPath(a.Gifs.P1080),
		},
		Instructions: a.Instructions,
		Equipments:   a.Equipments,
		// Point the client at our own backend proxy instead of the raw RapidAPI
		// URL, so the API key never has to live in the mobile/web app.
		MuscleHeatmapUrl: buildHeatmapProxyPath(primaryMuscle),
	}, nil
}

// FetchHeatmap downloads the muscle-activation heatmap image for the given
// muscle from the muscle-visualizer RapidAPI and returns the raw bytes plus the
// upstream Content-Type. The RapidAPI key stays server-side.
func (c *ExerciseClient) FetchHeatmap(muscle string) ([]byte, string, error) {
	if c.apiKey == "" {
		return nil, "", fmt.Errorf("rapidapi: RAPIDAPI_KEY is not set in environment")
	}

	reqURL := buildHeatmapURL(muscle)
	if reqURL == "" {
		return nil, "", fmt.Errorf("rapidapi: muscle is required")
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("rapidapi: failed to build heatmap request: %w", err)
	}
	req.Header.Add("x-rapidapi-key", c.apiKey)
	req.Header.Add("x-rapidapi-host", heatmapAPIHost)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("rapidapi: heatmap request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("rapidapi: heatmap returned %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("rapidapi: failed to read heatmap body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

// FetchAsset downloads an exercise image/GIF from the public CDN and returns the
// raw bytes + Content-Type. `token` is the base64url-encoded original CDN URL
// produced by buildAssetProxyPath. The host is validated against the allowlist
// to prevent the proxy from being abused as an open SSRF relay. No RapidAPI key
// is needed — these assets are public.
func (c *ExerciseClient) FetchAsset(token string) ([]byte, string, error) {
	rawURL, err := decodeAssetToken(token)
	if err != nil {
		return nil, "", fmt.Errorf("rapidapi: invalid asset token: %w", err)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != assetCDNHost {
		return nil, "", fmt.Errorf("rapidapi: asset host not allowed")
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("rapidapi: failed to build asset request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("rapidapi: asset request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("rapidapi: asset returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("rapidapi: failed to read asset body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return data, contentType, nil
}

// ─── Private Helpers ──────────────────────────────────────────────────────────

func (c *ExerciseClient) setHeaders(req *http.Request) {
	req.Header.Add("x-rapidapi-key", c.apiKey)
	req.Header.Add("x-rapidapi-host", ascendAPIHost)
	req.Header.Add("Content-Type", "application/json")
}

// ascendBodyPartMap translates the app's body-part tab names into the vocabulary
// the AscendAPI uses (ExerciseDB taxonomy). Tabs that cover multiple AscendAPI
// body parts map to a comma-separated list, which the API accepts in one call.
var ascendBodyPartMap = map[string]string{
	"chest":      "chest",
	"back":       "back",
	"shoulder":   "shoulders",
	"shoulders":  "shoulders",
	"leg":        "upper legs,lower legs",
	"legs":       "upper legs,lower legs",
	"arm":        "upper arms,lower arms",
	"arms":       "upper arms,lower arms",
	"core":       "waist",
	"abs":        "waist",
	"abdominals": "waist",
}

// mapBodyPartsForAscend converts a tab/group name to the AscendAPI body-part
// query. Unknown values pass through unchanged so already-valid AscendAPI
// vocabulary (e.g. "upper legs") keeps working.
func mapBodyPartsForAscend(parts string) string {
	if mapped, ok := ascendBodyPartMap[strings.ToLower(strings.TrimSpace(parts))]; ok {
		return mapped
	}
	return parts
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

// buildHeatmapProxyPath returns the path (relative to the API base URL) that the
// client should hit to load the heatmap through our backend proxy. Empty when no
// muscle is available.
func buildHeatmapProxyPath(muscle string) string {
	if muscle == "" {
		return ""
	}
	return "/exercises/heatmap?muscle=" + url.QueryEscape(muscle)
}

// buildAssetProxyPath rewrites a CDN image/GIF URL into a backend proxy path the
// client loads same-origin (avoids the CDN's missing CORS headers). Only CDN
// URLs are rewritten; anything else (empty, local catalog URLs) passes through
// unchanged. The original URL is base64url-encoded so it survives as a single
// query value without nested-encoding issues.
func buildAssetProxyPath(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host != assetCDNHost {
		return rawURL
	}
	token := base64.RawURLEncoding.EncodeToString([]byte(rawURL))
	return "/exercises/asset?u=" + token
}

// decodeAssetToken reverses buildAssetProxyPath's base64url encoding.
func decodeAssetToken(token string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
