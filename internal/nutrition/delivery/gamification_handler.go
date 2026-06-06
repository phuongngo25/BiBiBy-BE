package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/middleware"
)

// GamificationHandler wires all gamification-related routes onto the given router group.
type GamificationHandler struct {
	uc domain.GamificationUseCase
}

// NewGamificationHandler registers all gamification routes on the provided RouterGroup.
func NewGamificationHandler(rg *gin.RouterGroup, uc domain.GamificationUseCase) {
	h := &GamificationHandler{uc: uc}

	rg.GET("/gamification/achievements", h.GetAchievements)
}

// GetAchievements returns all available achievement definitions and the user's unlocked status.
// GET /api/v1/gamification/achievements
func (h *GamificationHandler) GetAchievements(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	response, err := h.uc.GetAchievements(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
