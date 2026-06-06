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
	ActivityLevel     ActivityLevel `json:"activity_level"`
	BMR               float64   `json:"bmr"`
	TDEE              float64   `json:"tdee"`
	GoalType          GoalType    `json:"goal_type"`
	Timezone          string    `json:"timezone"           gorm:"default:'UTC'"`
	WeeklyCalorieBudget float64 `json:"weekly_calorie_budget"`
	DietaryPreference string    `json:"dietary_preference"`
	Allergies           string    `json:"allergies"           gorm:"type:text"`
	AllergiesBidx       string    `json:"-"                   gorm:"index"`
	MedicalConditions   string    `json:"medical_conditions"  gorm:"type:text"`
	MedicalConditionsBidx string  `json:"-"                   gorm:"index"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// RefreshToken represents a long-lived token used to obtain new access tokens.
type RefreshToken struct {
	ID                  uuid.UUID `json:"id"                     gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID              uuid.UUID `json:"user_id"                gorm:"index;not null;constraint:OnDelete:CASCADE;"`
	TokenHash           string    `json:"-"                      gorm:"uniqueIndex;not null"`
	FamilyID            uuid.UUID `json:"family_id"              gorm:"index;not null"`
	ExpiresAt           time.Time `json:"expires_at"             gorm:"not null"`
	Revoked             bool      `json:"revoked"                gorm:"default:false"`
	ReplacedByTokenHash *string   `json:"-"`
	CreatedAt           time.Time `json:"created_at"             gorm:"autoCreateTime"`
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

// AuthResponse is the response payload returned after successful auth (Register/Login/Refresh).
type AuthResponse struct {
	Token        string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

// RefreshRequest is the input payload for exchanging a refresh token for new tokens.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UpdateProfileRequest is the input payload for updating a user's health profile.
// DOB is accepted as a plain date string ("YYYY-MM-DD") to avoid Go's strict RFC3339
// JSON unmarshaling rejecting the format Flutter sends.
type UpdateProfileRequest struct {
	FullName          *string    `json:"full_name"`
	HeightCm          *float64   `json:"height_cm"`
	WeightKg          *float64   `json:"weight_kg"`
	DateOfBirth       *string    `json:"date_of_birth"` // accept as plain "YYYY-MM-DD" or ISO string
	DOB               *time.Time `json:"-"`             // populated by handler after parsing
	Gender            string     `json:"gender"`
	ActivityLevel     string     `json:"activity_level"`
	GoalType          *string    `json:"goal_type"`
	DietaryPreference string     `json:"dietary_preference"`
	Allergies         string     `json:"allergies"`
	MedicalConditions string     `json:"medical_conditions"`
	BMR               *float64   `json:"-"`
	TDEE              *float64   `json:"-"`
}

// UserRepository defines the data access boundary for the User entity.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, req *UpdateProfileRequest) error

	// Refresh Token methods
	SaveRefreshToken(ctx context.Context, rt *RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, oldHash string, replacedByHash *string) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error
}

// UserUseCase defines the application-level business logic boundary for user operations.
type UserUseCase interface {
	Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	RefreshTokens(ctx context.Context, req *RefreshRequest) (*AuthResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req *UpdateProfileRequest) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*User, error)
	GetTargets(ctx context.Context, userID uuid.UUID) (*UserTargetsResponse, error)
}

// ─── Nutritional Targets DTOs ─────────────────────────────────────────────────

// TargetValue holds current (logged) vs recommended (DRI-based) values for a macro.
type TargetValue struct {
	Current float64 `json:"current"`
	Target  float64 `json:"target"`
}

// MicroTarget represents a single micronutrient's target and current intake.
type MicroTarget struct {
	Name    string  `json:"name"`
	Current float64 `json:"current"`
	Target  float64 `json:"target"`
	Unit    string  `json:"unit"`
}

// UserTargetsResponse is the full response payload for GET /profile/targets.
type UserTargetsResponse struct {
	TotalCalories  TargetValue            `json:"total_calories"`
	Burned         float64                `json:"burned_calories"` // Total burned via workout logs today
	Macronutrients map[string]TargetValue `json:"macronutrients"`
	Micronutrients []MicroTarget          `json:"micronutrients"`
}
