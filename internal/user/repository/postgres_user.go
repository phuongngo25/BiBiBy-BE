package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nutrix-backend/internal/domain"
)

// postgresUserRepository is the Postgres-backed implementation of UserRepository.
type postgresUserRepository struct {
	db *gorm.DB
}

// NewPostgresUserRepository creates a new UserRepository backed by GORM.
// The *gorm.DB dependency is injected at construction time (Dependency Injection).
func NewPostgresUserRepository(db *gorm.DB) domain.UserRepository {
	return &postgresUserRepository{db: db}
}

// Create persists a new User record to the database.
func (r *postgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	result := r.db.WithContext(ctx).Create(user)
	return result.Error
}

// GetByEmail retrieves a User by their email address.
// Returns domain.ErrUserNotFound if no record exists.
func (r *postgresUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByUsername retrieves a User by their username.
// Returns domain.ErrUserNotFound if no record exists.
func (r *postgresUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByID retrieves a User by their primary key.
// Uses COALESCE to substitute sensible defaults for metric columns that may be
// NULL for accounts created before the health-profile fields were added.
func (r *postgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Select(`id, username, email,
			COALESCE(height_cm, 170) AS height_cm,
			COALESCE(weight_kg, 70)  AS weight_kg,
			gender, activity_level, dietary_preference,
			allergies, medical_conditions,
			created_at, updated_at`).
		First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// UpdateProfile updates a user's full health profile.
// Uses a map to only update fields that are present in the request,
// ensuring pointer-nil fields are safely skipped (zero-value protection).
func (r *postgresUserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, req *domain.UpdateProfileRequest) error {
	updates := map[string]interface{}{
		"dietary_preference": req.DietaryPreference,
		"allergies":          req.Allergies,
		"medical_conditions": req.MedicalConditions,
		"gender":             req.Gender,
		"activity_level":     req.ActivityLevel,
		"goal_type":          req.GoalType,
	}
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	// Only update numeric fields if the client provided a non-nil value
	if req.HeightCm != nil {
		updates["height_cm"] = *req.HeightCm
	}
	if req.WeightKg != nil {
		updates["weight_kg"] = *req.WeightKg
	}
	if req.DOB != nil {
		updates["dob"] = *req.DOB
	}
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Updates(updates).Error
}
