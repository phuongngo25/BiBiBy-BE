package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// User represents the core domain entity for a NutriX platform user.
// It combines authentication fields with biometric/health data.
type User struct {
	ID                uuid.UUID `json:"id"                  gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Username          string    `json:"username"            gorm:"uniqueIndex;not null"`
	Email             string    `json:"email"               gorm:"uniqueIndex;not null"`
	Password          string    `json:"-"                   gorm:"column:password_hash;not null"`
	FullName          string    `json:"full_name"`
	HeightCm          float64   `json:"height_cm"`
	WeightKg          float64   `json:"weight_kg"`
	DOB               *time.Time `json:"dob"                gorm:"type:date"`
	Gender            string    `json:"gender"`
	ActivityLevel     string    `json:"activity_level"`
	BMR               float64   `json:"bmr"`
	TDEE              float64   `json:"tdee"`
	GoalType          string    `json:"goal_type"`
	WeeklyCalorieBudget float64 `json:"weekly_calorie_budget"`
	DietaryPreference string    `json:"dietary_preference"`
	Allergies         string    `json:"allergies"           gorm:"type:text"`
	MedicalConditions string    `json:"medical_conditions"  gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RegisterRequest is the input payload for a new user registration.
type RegisterRequest struct {
	Username          string     `json:"username"           binding:"required"`
	Email             string     `json:"email"              binding:"required,email"`
	Password          string     `json:"password"           binding:"required,min=6"`
	FullName          string     `json:"full_name"`
	HeightCm          float64    `json:"height_cm"`
	WeightKg          float64    `json:"weight_kg"`
	DOB               *time.Time `json:"dob"`
	Gender            string     `json:"gender"`
	ActivityLevel     string     `json:"activity_level"`
	DietaryPreference string     `json:"dietary_preference"`
	MedicalConditions string     `json:"medical_conditions"`
}

// LoginRequest is the input payload for authenticating an existing user.
// Identifier accepts either an email address or a username.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password"   binding:"required"`
}

// AuthResponse is the response payload returned after successful auth (Register/Login).
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// UpdateProfileRequest is the input payload for updating a user's health profile.
type UpdateProfileRequest struct {
	FullName          *string    `json:"full_name"`
	HeightCm          *float64   `json:"height_cm"`
	WeightKg          *float64   `json:"weight_kg"`
	DOB               *time.Time `json:"dob"`
	Gender            string     `json:"gender"`
	ActivityLevel     string     `json:"activity_level"`
	GoalType          string     `json:"goal_type"`
	DietaryPreference string     `json:"dietary_preference"`
	Allergies         string     `json:"allergies"`
	MedicalConditions string     `json:"medical_conditions"`
}

// UserRepository defines the data access boundary for the User entity.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, req *UpdateProfileRequest) error
}

// UserUseCase defines the application-level business logic boundary for user operations.
type UserUseCase interface {
	Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req *UpdateProfileRequest) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*User, error)
}
