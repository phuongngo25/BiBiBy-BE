package delivery

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/middleware"
)

// NutritionHandler wires all nutrition-related routes onto the given router group.
type NutritionHandler struct {
	uc domain.NutritionUseCase
}

// NewNutritionHandler registers all nutrition routes on the provided RouterGroup.
func NewNutritionHandler(rg *gin.RouterGroup, uc domain.NutritionUseCase) {
	h := &NutritionHandler{uc: uc}

	rg.GET("/nutrition/foods/search", h.SearchFoods)
	rg.POST("/nutrition/foods/upload-image", h.UploadFoodImage)
	rg.POST("/nutrition/foods/estimate", h.EstimateNutrition)
	rg.POST("/nutrition/foods", h.CreateFood)
	rg.GET("/nutrition/daily-plan", h.GetDailyPlan)
	rg.POST("/nutrition/log-meal", h.LogMeal)
	rg.POST("/nutrition/log-water", h.LogWater)
	rg.GET("/nutrition/search-spoonacular", h.SearchSpoonacular)
	rg.GET("/nutrition/search-by-nutrients", h.SearchByNutrients)
	rg.GET("/nutrition/search-by-ingredients", h.SearchByIngredients)
	rg.GET("/nutrition/analytics/day", h.GetDayAnalytics)
	rg.GET("/nutrition/analytics/weekly", h.GetWeeklyAnalytics)
	rg.GET("/nutrition/analytics/monthly", h.GetMonthlyAnalytics)
	rg.GET("/nutrition/analytics/streak", h.GetStreak)
	rg.PUT("/nutrition/logs/:id", h.UpdateFoodLog)

	// Orchestrator Job Status API
	rg.GET("/jobs/:id", h.GetJobStatus)

	// Explainability API (Sprint 2) - Removed (Deprecated in AI Server v1)

	// Optimizer API (Sprint 4)
	rg.POST("/nutrition/recommendations", h.GetRecommendations)

	// Thresholds & Feedback API (Migration Epic 1)
	rg.GET("/nutrition/thresholds", h.GetThresholds)
	rg.POST("/nutrition/feedback/correction", h.SubmitFoodCorrection)
	rg.POST("/nutrition/feedback/acceptance", h.SubmitFoodAcceptance)
	rg.POST("/nutrition/feedback/viewed", h.SubmitFoodViewed)
	rg.POST("/nutrition/meal/validate", h.ValidateMeal)

	// Planner API
	rg.POST("/planner/reoptimize", h.PlannerReoptimize)
	rg.POST("/planner/explain", h.PlannerExplain)
	rg.POST("/planner/weekly-plan", h.GenerateWeeklyPlan)
}

// GetRecommendations godoc
// POST /api/v1/nutrition/recommendations
func (h *NutritionHandler) GetRecommendations(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req domain.GetRecommendationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	recommendations, err := h.uc.GetRecommendations(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, recommendations)
}

// SearchFoods godoc
// GET /api/v1/nutrition/foods/search?q=<keyword>
// Queries local DB first; falls back to Spoonacular on cache-miss.
func (h *NutritionHandler) SearchFoods(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param 'q' is required"})
		return
	}

	foods, err := h.uc.SearchFoods(c.Request.Context(), keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, foods)
}

// SearchSpoonacular godoc
// GET /api/v1/nutrition/search-spoonacular?q=&diet=&intolerances=&maxCarbs=
func (h *NutritionHandler) SearchSpoonacular(c *gin.Context) {
	query := c.Query("q")
	diet := c.Query("diet")
	intolerances := c.Query("intolerances")
	maxCarbs, _ := strconv.Atoi(c.Query("maxCarbs"))

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param 'q' is required"})
		return
	}

	foods, err := h.uc.SearchSpoonacular(c.Request.Context(), query, diet, intolerances, maxCarbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, foods)
}

// SearchByNutrients godoc
// GET /api/v1/nutrition/search-by-nutrients?minProtein=&maxFat=&minCalories=&maxCalories=
func (h *NutritionHandler) SearchByNutrients(c *gin.Context) {
	minProtein, _ := strconv.ParseFloat(c.Query("minProtein"), 64)
	maxFat, _ := strconv.ParseFloat(c.Query("maxFat"), 64)
	minCalories, _ := strconv.ParseFloat(c.Query("minCalories"), 64)
	maxCalories, _ := strconv.ParseFloat(c.Query("maxCalories"), 64)

	foods, err := h.uc.SearchByNutrients(c.Request.Context(), minProtein, maxFat, minCalories, maxCalories)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, foods)
}

// SearchByIngredients godoc
// GET /api/v1/nutrition/search-by-ingredients?ingredients=chicken,rice
func (h *NutritionHandler) SearchByIngredients(c *gin.Context) {
	ingredients := c.Query("ingredients")
	if ingredients == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query param 'ingredients' is required"})
		return
	}

	foods, err := h.uc.SearchByIngredients(c.Request.Context(), ingredients)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, foods)
}

// CreateFood godoc
// POST /api/v1/nutrition/foods
func (h *NutritionHandler) CreateFood(c *gin.Context) {
	var req domain.CreateFoodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	food, err := h.uc.CreateFood(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, food)
}

// LogMeal godoc
// POST /api/v1/nutrition/log-meal
func (h *NutritionHandler) LogMeal(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req domain.LogMealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mealLog, err := h.uc.LogMeal(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mealLog)
}

// LogWater godoc
// POST /api/v1/nutrition/log-water
func (h *NutritionHandler) LogWater(c *gin.Context) {
	log.Println("[WATER] handler entered")
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	body, _ := io.ReadAll(c.Request.Body)
	log.Printf("[WATER] raw body=%s", string(body))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req domain.LogWaterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[WATER] BindJSON error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.uc.LogWater(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidQuantity) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// GetDailyPlan godoc
// GET /api/v1/nutrition/daily-plan?date=YYYY-MM-DD
func (h *NutritionHandler) GetDailyPlan(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	log.Printf("[DAILY_PLAN] requestedDate=%s", dateStr)

	// Validate format early so we can return a clean 400
	if _, errParse := time.Parse("2006-01-02", dateStr); errParse != nil {
		log.Printf("[DAILY_PLAN] invalid date format: %v", errParse)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, expected YYYY-MM-DD"})
		return
	}

	plan, err := h.uc.GetDailyPlan(c.Request.Context(), userID, dateStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, plan)
}

// UploadFoodImage godoc
// POST /api/v1/nutrition/foods/upload-image
// Accepts multipart/form-data with field "image". Saves to ./uploads/foods/{uuid}.jpg
func (h *NutritionHandler) UploadFoodImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}

	uploadDir := "uploads/foods"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create upload directory"})
		return
	}

	filename := uuid.New().String() + filepath.Ext(file.Filename)
	savePath := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"image_url": "/" + savePath})
}

// GetWeeklyAnalytics godoc
// GET /api/v1/nutrition/analytics/weekly
// Returns WeeklyAnalyticsResponse including structured range, sum statistics, and streak.
func (h *NutritionHandler) GetWeeklyAnalytics(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}
	isCalendar := c.Query("type") == "calendar"

	analytics, err := h.uc.GetWeeklyAnalytics(c.Request.Context(), userID, days, isCalendar)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, analytics)
}

// GetDayAnalytics godoc
// GET /api/v1/nutrition/analytics/day?date=YYYY-MM-DD
func (h *NutritionHandler) GetDayAnalytics(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	if _, errParse := time.Parse("2006-01-02", dateStr); errParse != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, expected YYYY-MM-DD"})
		return
	}

	analytics, err := h.uc.GetDayAnalytics(c.Request.Context(), userID, dateStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, analytics)
}

// GetMonthlyAnalytics godoc
// GET /api/v1/nutrition/analytics/monthly?month=YYYY-MM
func (h *NutritionHandler) GetMonthlyAnalytics(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	monthStr := c.Query("month")
	if monthStr == "" {
		monthStr = time.Now().Format("2006-01")
	}

	if _, errParse := time.Parse("2006-01", monthStr); errParse != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month format, expected YYYY-MM"})
		return
	}

	analytics, err := h.uc.GetMonthlyAnalytics(c.Request.Context(), userID, monthStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, analytics)
}

// GetStreak godoc
// GET /api/v1/nutrition/analytics/streak
func (h *NutritionHandler) GetStreak(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	streak, err := h.uc.GetStreak(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"current_streak":      streak.CurrentStreak,
		"longest_streak":      streak.LongestStreak,
		"last_evaluated_date": streak.LastEvaluatedDate.Format("2006-01-02"),
	})
}

// UpdateFoodLog godoc
// PUT /api/v1/nutrition/logs/:id
func (h *NutritionHandler) UpdateFoodLog(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	logID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid log ID format"})
		return
	}

	var req struct {
		QuantityGrams float64 `json:"quantity_grams" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedLog, err := h.uc.UpdateFoodLog(c.Request.Context(), userID, logID, req.QuantityGrams)
	if err != nil {
		if errors.Is(err, domain.ErrLogNotFound) {
			// Anti-enumeration: return 404 if not found OR not owned by user
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domain.ErrInvalidQuantity) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedLog)
}

// GetJobStatus godoc
// GET /api/v1/jobs/:id
func (h *NutritionHandler) GetJobStatus(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job id is required"})
		return
	}

	// Assuming UseCase proxy for JobStore lookup
	job, err := h.uc.GetJobStatus(c.Request.Context(), jobID)
	if err != nil {
		// Replace this with standard err check later
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// Strict Production API Contract mapping
	response := gin.H{
		"id":         job.ID,
		"job_type":   job.Type, // Explicit mapping
		"status":     job.Status,
		"done":       job.Done,
		"updated_at": job.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"), // Strict RFC3339
	}

	if job.Error != "" {
		response["error"] = job.Error
	}

	c.JSON(http.StatusOK, response)
}

func (h *NutritionHandler) EstimateNutrition(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open image file"})
		return
	}
	defer src.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image file"})
		return
	}
	imageBytes := buf.Bytes()
	log.Printf("[CV] EstimateNutrition upload filename=%s multipart_size=%d bytes_read=%d detected_type=%s",
		file.Filename, file.Size, len(imageBytes), http.DetectContentType(imageBytes))

	result, err := h.uc.EstimateNutrition(c.Request.Context(), imageBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetThresholds godoc
func (h *NutritionHandler) GetThresholds(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	snapshot, err := h.uc.GetThresholdSnapshot(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

func (h *NutritionHandler) handleFeedback(c *gin.Context, action string) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req domain.FoodFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate UserAction (T-B4)
	if req.UserAction != "" && req.UserAction != action {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user action in payload"})
		return
	}
	req.UserAction = action

	err = h.uc.SubmitFoodFeedback(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// SubmitFoodCorrection godoc
func (h *NutritionHandler) SubmitFoodCorrection(c *gin.Context) {
	h.handleFeedback(c, "CORRECTION")
}

// SubmitFoodAcceptance godoc
func (h *NutritionHandler) SubmitFoodAcceptance(c *gin.Context) {
	h.handleFeedback(c, "ACCEPTANCE")
}

// SubmitFoodViewed godoc
func (h *NutritionHandler) SubmitFoodViewed(c *gin.Context) {
	h.handleFeedback(c, "VIEWED")
}

// PlannerReoptimize godoc
// POST /api/v1/planner/reoptimize
func (h *NutritionHandler) PlannerReoptimize(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req domain.ReoptimizeWeeklyPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	resp, err := h.uc.ReoptimizePlan(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, domain.ErrAllCandidatesRejected) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.WeeklyPlanResponseDTO)
}

// PlannerExplain godoc
func (h *NutritionHandler) PlannerExplain(c *gin.Context) {
	h.validateMeal(c)
}

// ValidateMeal godoc
// POST /api/v1/nutrition/meal/validate
func (h *NutritionHandler) ValidateMeal(c *gin.Context) {
	h.validateMeal(c)
}

func (h *NutritionHandler) validateMeal(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req domain.AnalyzeMealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.uc.AnalyzeMeal(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GenerateWeeklyPlan godoc
// POST /api/v1/planner/weekly-plan
func (h *NutritionHandler) GenerateWeeklyPlan(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req domain.GenerateWeeklyPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	resp, err := h.uc.GenerateWeeklyPlan(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, domain.ErrAllCandidatesRejected) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.WeeklyPlanResponseDTO)
}
