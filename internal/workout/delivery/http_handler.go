package delivery

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/middleware"
)

// WorkoutHandler manages the REST endpoints for the workout module.
type WorkoutHandler struct {
	useCase domain.WorkoutUseCase
}

// NewWorkoutHandler registers the workout routes onto the provided protected RouterGroup.
//
//	GET  /api/v1/exercises?bodyparts=pectorals
//	GET  /api/v1/exercises/:id
//	POST /api/v1/workouts/log
func NewWorkoutHandler(rg *gin.RouterGroup, useCase domain.WorkoutUseCase) {
	handler := &WorkoutHandler{useCase: useCase}

	workoutGroup := rg.Group("/exercises")
	{
		workoutGroup.GET("", handler.GetExercisesByBodyParts)
		workoutGroup.GET("/:id", handler.GetExerciseByID)
	}

	logGroup := rg.Group("/workouts")
	{
		logGroup.POST("/log", handler.LogWorkout)
	}
}

// GetExercisesByBodyParts handles GET /api/v1/exercises?bodyparts={parts}
func (h *WorkoutHandler) GetExercisesByBodyParts(c *gin.Context) {
	bodyparts := strings.TrimSpace(c.Query("bodyparts"))
	if bodyparts == "" {
		bodyparts = strings.TrimSpace(c.Query("muscle")) // fallback
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

// LogWorkout handles POST /api/v1/workouts/log
func (h *WorkoutHandler) LogWorkout(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	uid, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID format in token"})
		return
	}

	var req domain.LogWorkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workoutLog, err := h.useCase.LogWorkout(c.Request.Context(), uid, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, workoutLog)
}

