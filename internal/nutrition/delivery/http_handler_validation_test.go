package delivery_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/delivery"
)

// mockNutritionUseCase is a dummy implementation just to bypass setup errors
type mockNutritionUseCase struct {
	domain.NutritionUseCase
}

func TestLogMeal_Validation_MaxGrams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Bypass middleware by injecting dummy user ID directly
	router.Use(func(c *gin.Context) {
		c.Set("userID", uuid.New().String())
		c.Next()
	})

	uc := &mockNutritionUseCase{}
	delivery.NewNutritionHandler(router.Group("/api/v1"), uc)

	tests := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{
			name:       "Valid grams",
			payload:    `{"food_id":"550e8400-e29b-41d4-a716-446655440000", "quantity_grams": 4999.0, "meal_type": "lunch", "consumed_date": "2026-06-05"}`,
			expectCode: http.StatusInternalServerError, // It passes validation, reaches UseCase which is mocked, returning 500 or 201 (if mocked correctly)
		},
		{
			name:       "Exceed max grams (5001)",
			payload:    `{"food_id":"550e8400-e29b-41d4-a716-446655440000", "quantity_grams": 5001.0, "meal_type": "lunch", "consumed_date": "2026-06-05"}`,
			expectCode: http.StatusBadRequest, // Validation should fail!
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/nutrition/log-meal", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code == http.StatusBadRequest && tc.expectCode != http.StatusBadRequest {
				t.Errorf("expected passing validation, got 400: %s", w.Body.String())
			}
			if tc.expectCode == http.StatusBadRequest && w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 Bad Request due to validation, got %d", w.Code)
			}
		})
	}
}
