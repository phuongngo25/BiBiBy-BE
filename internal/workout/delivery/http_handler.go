package delivery

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"nutrix-backend/internal/domain"
)

// WorkoutHandler manages the REST endpoints for the workout module.
type WorkoutHandler struct {
	useCase domain.WorkoutUseCase
}

// NewWorkoutHandler registers the workout routes onto the provided protected RouterGroup.
//
//	GET /api/v1/exercises?bodyparts=pectorals
//	GET /api/v1/exercises/:id
func NewWorkoutHandler(rg *gin.RouterGroup, useCase domain.WorkoutUseCase) {
	handler := &WorkoutHandler{useCase: useCase}

	workoutGroup := rg.Group("/exercises")
	{
		workoutGroup.GET("", handler.GetExercisesByBodyParts)
		workoutGroup.GET("/:id", handler.GetExerciseByID)
	}
}

// GetExercisesByBodyParts handles GET /api/v1/exercises?bodyparts={parts}
//
// Query params:
//   - bodyparts (required): target body part, e.g. "pectorals"
//   - muscle (legacy): fallback for older clients querying ?muscle=chest
func (h *WorkoutHandler) GetExercisesByBodyParts(c *gin.Context) {
	bodyparts := strings.TrimSpace(c.Query("bodyparts"))
	if bodyparts == "" {
		// Fallback for Flutter legacy compatibility
		bodyparts = strings.TrimSpace(c.Query("muscle"))
	}

	if bodyparts == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "query parameter 'bodyparts' (or 'muscle') is required",
		})
		return
	}

	exercises, err := h.useCase.GetExercisesByBodyParts(c.Request.Context(), bodyparts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bodyparts": bodyparts,
		"count":     len(exercises),
		"exercises": exercises,
	})
}

// GetExerciseByID handles GET /api/v1/exercises/:id
func (h *WorkoutHandler) GetExerciseByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exercise ID is required in URL path"})
		return
	}

	detail, err := h.useCase.GetExerciseByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}
