package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"nutrix-backend/internal/domain"
)

type userUseCase struct {
	repo               domain.UserRepository
	jwtSecret          string
	jwtExpirationHours int
}

// NewUserUseCase creates a UserUseCase with the provided repository and JWT config.
func NewUserUseCase(repo domain.UserRepository, jwtSecret string, jwtExpirationHours int) domain.UserUseCase {
	return &userUseCase{
		repo:               repo,
		jwtSecret:          jwtSecret,
		jwtExpirationHours: jwtExpirationHours,
	}
}

// Register creates a new user account. Hashes the password with bcrypt before storing.
func (u *userUseCase) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.AuthResponse, error) {
	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, domain.ErrInternalServerError
	}

	user := &domain.User{
		Username:          req.Username,
		Email:             strings.ToLower(req.Email),
		Password:          string(hashed),
		FullName:          req.FullName,
		HeightCm:          req.HeightCm,
		WeightKg:          req.WeightKg,
		DOB:               req.DOB,
		Gender:            req.Gender,
		ActivityLevel:     req.ActivityLevel,
		DietaryPreference: req.DietaryPreference,
		MedicalConditions: req.MedicalConditions,
	}

	if err := u.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	token, err := u.generateToken(user.ID)
	if err != nil {
		return nil, domain.ErrInternalServerError
	}

	return &domain.AuthResponse{Token: token, User: *user}, nil
}

// Login authenticates a user by email or username + password.
func (u *userUseCase) Login(ctx context.Context, req *domain.LoginRequest) (*domain.AuthResponse, error) {
	// Try email first, then username
	var user *domain.User
	var err error

	if strings.Contains(req.Identifier, "@") {
		user, err = u.repo.GetByEmail(ctx, strings.ToLower(req.Identifier))
	} else {
		user, err = u.repo.GetByUsername(ctx, req.Identifier)
	}

	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	token, err := u.generateToken(user.ID)
	if err != nil {
		return nil, domain.ErrInternalServerError
	}

	return &domain.AuthResponse{Token: token, User: *user}, nil
}

// UpdateProfile delegates profile updates to the repository.
func (u *userUseCase) UpdateProfile(ctx context.Context, userID uuid.UUID, req *domain.UpdateProfileRequest) error {
	return u.repo.UpdateProfile(ctx, userID, req)
}

// GetProfile retrieves the full user profile by ID.
func (u *userUseCase) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return u.repo.GetByID(ctx, userID)
}

// generateToken produces a signed JWT with the user's UUID in the "sub" claim.
func (u *userUseCase) generateToken(userID uuid.UUID) (string, error) {
	expiry := time.Duration(u.jwtExpirationHours) * time.Hour
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(expiry).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(u.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}
	return signed, nil
}
