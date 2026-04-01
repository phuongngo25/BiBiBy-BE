package delivery

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

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
	rg.POST("/nutrition/foods", h.CreateFood)
	rg.GET("/nutrition/daily-plan", h.GetDailyPlan)
	rg.POST("/nutrition/log-meal", h.LogMeal)
	rg.GET("/nutrition/search-spoonacular", h.SearchSpoonacular)
	rg.GET("/nutrition/search-by-nutrients", h.SearchByNutrients)
	rg.GET("/nutrition/search-by-ingredients", h.SearchByIngredients)
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
	query        := c.Query("q")
	diet         := c.Query("diet")
	intolerances := c.Query("intolerances")
	maxCarbs, _  := strconv.Atoi(c.Query("maxCarbs"))

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
	minProtein,  _ := strconv.ParseFloat(c.Query("minProtein"),  64)
	maxFat,      _ := strconv.ParseFloat(c.Query("maxFat"),      64)
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

// GetDailyPlan godoc
// GET /api/v1/nutrition/daily-plan
func (h *NutritionHandler) GetDailyPlan(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	plan, err := h.uc.GetDailyPlan(c.Request.Context(), userID)
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
