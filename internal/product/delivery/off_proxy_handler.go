package delivery

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// offBaseURL is the public OpenFoodFacts host. We proxy its product/search
// endpoints server-side so the Flutter web client can call them same-origin —
// the browser blocks the direct cross-origin call (CORS) and forbids the
// custom User-Agent header OpenFoodFacts asks clients to send.
const offBaseURL = "https://world.openfoodfacts.org"

// OFFProxyHandler is a thin, transparent reverse-proxy for OpenFoodFacts.
// It forwards the client's query string verbatim and copies OFF's status code
// and body back unchanged, so the existing client-side DTOs keep parsing as-is
// (including the 404 "product not found" path).
type OFFProxyHandler struct {
	client    *http.Client
	userAgent string
}

// NewOFFProxyHandler registers the OpenFoodFacts proxy routes:
//
//	GET /api/v1/products/off/barcode/:barcode
//	GET /api/v1/products/off/search
func NewOFFProxyHandler(rg *gin.RouterGroup, userAgent string) {
	h := &OFFProxyHandler{
		client:    &http.Client{Timeout: 20 * time.Second},
		userAgent: userAgent,
	}

	g := rg.Group("/products/off")
	{
		g.GET("/barcode/:barcode", h.GetByBarcode)
		g.GET("/search", h.Search)
	}
}

// GetByBarcode proxies GET /api/v2/product/{barcode}.
func (h *OFFProxyHandler) GetByBarcode(c *gin.Context) {
	barcode := c.Param("barcode")
	if barcode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "barcode is required"})
		return
	}
	target := fmt.Sprintf("%s/api/v2/product/%s", offBaseURL, url.PathEscape(barcode))
	if rq := c.Request.URL.RawQuery; rq != "" {
		target += "?" + rq
	}
	h.forward(c, target)
}

// Search proxies GET /cgi/search.pl.
func (h *OFFProxyHandler) Search(c *gin.Context) {
	target := offBaseURL + "/cgi/search.pl"
	if rq := c.Request.URL.RawQuery; rq != "" {
		target += "?" + rq
	}
	h.forward(c, target)
}

// forward performs the upstream GET and streams the response back verbatim.
func (h *OFFProxyHandler) forward(c *gin.Context, target string) {
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// OpenFoodFacts asks all clients to send a descriptive User-Agent.
	req.Header.Set("User-Agent", h.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "openfoodfacts request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read openfoodfacts response: " + err.Error()})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	// Preserve OFF's status code (notably 404 → product not found) and body so
	// the client's existing parsing/error handling stays valid.
	c.Data(resp.StatusCode, contentType, body)
}
