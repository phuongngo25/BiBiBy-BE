package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/middleware"
)

// UserHandler handles all auth and user profile routes.
type UserHandler struct {
	uc domain.UserUseCase
}

// NewUserHandler registers public auth routes (register, login) on the root router.
func NewUserHandler(r *gin.Engine, uc domain.UserUseCase) {
	h := &UserHandler{uc: uc}

	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
	}
}

// RegisterProfileRoutes registers protected profile routes on an authenticated RouterGroup.
func RegisterProfileRoutes(rg *gin.RouterGroup, uc domain.UserUseCase) {
	h := &UserHandler{uc: uc}
	rg.PUT("/users/profile", h.UpdateProfile)
	rg.GET("/users/profile", h.GetProfile)
}

// Register godoc
// POST /api/v1/auth/register
func (h *UserHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.uc.Register(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrUserAlreadyExists {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// Login godoc
// POST /api/v1/auth/login
func (h *UserHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.uc.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateProfile godoc
// PUT /api/v1/users/profile
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	var req domain.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.uc.UpdateProfile(c.Request.Context(), userID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "profile updated successfully"})
}

// GetProfile godoc
// GET /api/v1/users/profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID in token"})
		return
	}

	user, err := h.uc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if err == domain.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}
