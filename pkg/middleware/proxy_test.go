package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestClientIP_NoProxyTrusted verifies that when SetTrustedProxies(nil) is
// applied, Gin uses RemoteAddr directly and ignores any X-Forwarded-For header
// that an attacker could inject to spoof their IP and bypass rate limiting.
func TestClientIP_NoProxyTrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	// The fix under test: trust no proxies → RemoteAddr is the authoritative IP
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil) returned unexpected error: %v", err)
	}

	var capturedIP string
	r.GET("/ip", func(c *gin.Context) {
		capturedIP = c.ClientIP()
		c.String(http.StatusOK, capturedIP)
	})

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	// Attacker attempts to spoof IP via X-Forwarded-For
	req.Header.Set("X-Forwarded-For", "evil-spoofed-ip")
	// Real connection comes from 192.168.1.99
	req.RemoteAddr = "192.168.1.99:12345"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if capturedIP != "192.168.1.99" {
		t.Errorf("expected ClientIP=192.168.1.99 (from RemoteAddr), got %q\n"+
			"This means X-Forwarded-For spoofing is still possible!", capturedIP)
	}
}
