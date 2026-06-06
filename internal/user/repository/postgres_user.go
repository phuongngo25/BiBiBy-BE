package repository

import (
	"context"
	"errors"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/crypto"
)

// postgresUserRepository is the Postgres-backed implementation of UserRepository.
type postgresUserRepository struct {
	db               *gorm.DB
	encryptionKeys   map[string]string
	activeKeyVersion string
	hmacKey          string
	sf               singleflight.Group
}

// NewPostgresUserRepository creates a new UserRepository backed by GORM.
func NewPostgresUserRepository(db *gorm.DB, encryptionKeys map[string]string, activeVersion string, hmacKey string) domain.UserRepository {
	return &postgresUserRepository{
		db:               db,
		encryptionKeys:   encryptionKeys,
		activeKeyVersion: activeVersion,
		hmacKey:          hmacKey,
	}
}

// Create persists a new User record to the database.
func (r *postgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	// Encrypt sensitive fields before saving
	if err := r.encryptUserFields(user); err != nil {
		return err
	}

	err := r.db.WithContext(ctx).Create(user).Error

	// Restore plaintext to memory so caller doesn't see ciphertext
	_ = r.decryptUserFields(user)
	
	return err
}

// GetByEmail retrieves a User by their email address.
func (r *postgresUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	if needsRotation := r.decryptUserFields(&user); needsRotation {
		r.triggerAsyncRotation(user)
	}

	return &user, nil
}

// GetByUsername retrieves a User by their username.
func (r *postgresUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	if needsRotation := r.decryptUserFields(&user); needsRotation {
		r.triggerAsyncRotation(user)
	}

	return &user, nil
}

// GetByID retrieves a User by their primary key.
func (r *postgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	if needsRotation := r.decryptUserFields(&user); needsRotation {
		r.triggerAsyncRotation(user)
	}

	return &user, nil
}

// UpdateProfile updates a user's health profile with field-level encryption.
func (r *postgresUserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, req *domain.UpdateProfileRequest) error {
	updates := map[string]interface{}{
		"dietary_preference": req.DietaryPreference,
		"gender":             req.Gender,
		"activity_level":     req.ActivityLevel,
	}

	// Handle Encrypted Fields
	aad := crypto.BuildAAD(id.String())
	if req.Allergies != "" {
		enc, err := crypto.Encrypt(req.Allergies, r.encryptionKeys[r.activeKeyVersion], aad, r.activeKeyVersion)
		if err != nil {
			return err
		}
		updates["allergies"] = enc
		updates["allergies_bidx"] = crypto.BlindIndex(req.Allergies, r.hmacKey)
	}

	if req.MedicalConditions != "" {
		enc, err := crypto.Encrypt(req.MedicalConditions, r.encryptionKeys[r.activeKeyVersion], aad, r.activeKeyVersion)
		if err != nil {
			return err
		}
		updates["medical_conditions"] = enc
		updates["medical_conditions_bidx"] = crypto.BlindIndex(req.MedicalConditions, r.hmacKey)
	}

	if req.GoalType != nil {
		if *req.GoalType == "" {
			updates["goal_type"] = "Maintain Weight"
		} else {
			updates["goal_type"] = *req.GoalType
		}
	}
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	if req.HeightCm != nil {
		updates["height_cm"] = *req.HeightCm
	}
	if req.WeightKg != nil {
		updates["weight_kg"] = *req.WeightKg
	}
	if req.DOB != nil {
		updates["dob"] = *req.DOB
	}
	if req.BMR != nil {
		updates["bmr"] = *req.BMR
	}
	if req.TDEE != nil {
		updates["tdee"] = *req.TDEE
	}
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Updates(updates).Error
}

// ─── Private Crypto & Rotation Helpers ─────────────────────────────────────

func (r *postgresUserRepository) encryptUserFields(u *domain.User) error {
	aad := crypto.BuildAAD(u.ID.String())
	activeKey := r.encryptionKeys[r.activeKeyVersion]

	if u.Allergies != "" {
		enc, err := crypto.Encrypt(u.Allergies, activeKey, aad, r.activeKeyVersion)
		if err != nil {
			return err
		}
		u.Allergies = enc
		u.AllergiesBidx = crypto.BlindIndex(u.Allergies, r.hmacKey)
	}

	if u.MedicalConditions != "" {
		enc, err := crypto.Encrypt(u.MedicalConditions, activeKey, aad, r.activeKeyVersion)
		if err != nil {
			return err
		}
		u.MedicalConditions = enc
		u.MedicalConditionsBidx = crypto.BlindIndex(u.MedicalConditions, r.hmacKey)
	}

	return nil
}

func (r *postgresUserRepository) decryptUserFields(u *domain.User) bool {
	aad := crypto.BuildAAD(u.ID.String())
	needsRotation := false

	if u.Allergies != "" {
		plain, rot, err := crypto.Decrypt(u.Allergies, r.encryptionKeys, r.activeKeyVersion, aad)
		if err == nil {
			u.Allergies = plain
			if rot {
				needsRotation = true
			}
		}
	}

	if u.MedicalConditions != "" {
		plain, rot, err := crypto.Decrypt(u.MedicalConditions, r.encryptionKeys, r.activeKeyVersion, aad)
		if err == nil {
			u.MedicalConditions = plain
			if rot {
				needsRotation = true
			}
		}
	}

	return needsRotation
}

// triggerAsyncRotation kicks off a background task to upgrade fields to the latest key.
// Uses singleflight to ensure that multiple requests for the same user don't spam the DB.
func (r *postgresUserRepository) triggerAsyncRotation(u domain.User) {
	key := "reencrypt:user:" + u.ID.String()

	go func() {
		// singleflight prevents redundant updates for the same user session
		_, _, _ = r.sf.Do(key, func() (interface{}, error) {
			// Use a fresh background context with 10s timeout to ensure it persists past the request lifecycle
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			return nil, r.asyncReencryptProfile(ctx, u)
		})
	}()
}

// asyncReencryptProfile re-encrypts the user's data with the latest key version.
func (r *postgresUserRepository) asyncReencryptProfile(ctx context.Context, u domain.User) error {
	// Implement retry with exponential backoff
	maxAttempts := 3
	var err error

	for i := 0; i < maxAttempts; i++ {
		err = r.doReencrypt(ctx, u)
		if err == nil {
			return nil
		}
		
		log.Printf("[SecOps] Re-encryption failed for user %s, attempt %d/%d", u.ID, i+1, maxAttempts)
		backoff := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
		time.Sleep(backoff)
	}

	return err
}

func (r *postgresUserRepository) doReencrypt(ctx context.Context, u domain.User) error {
	// Prepare new encrypted values
	aad := crypto.BuildAAD(u.ID.String())
	activeKey := r.encryptionKeys[r.activeKeyVersion]

	updates := make(map[string]interface{})

	if u.Allergies != "" {
		enc, err := crypto.Encrypt(u.Allergies, activeKey, aad, r.activeKeyVersion)
		if err != nil {
			return err
		}
		updates["allergies"] = enc
		updates["allergies_bidx"] = crypto.BlindIndex(u.Allergies, r.hmacKey)
	}

	if u.MedicalConditions != "" {
		enc, err := crypto.Encrypt(u.MedicalConditions, activeKey, aad, r.activeKeyVersion)
		if err != nil {
			return err
		}
		updates["medical_conditions"] = enc
		updates["medical_conditions_bidx"] = crypto.BlindIndex(u.MedicalConditions, r.hmacKey)
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", u.ID).Updates(updates).Error
}

// ─── Refresh Token Methods ──────────────────────────────────────────────────

// SaveRefreshToken saves a new refresh token to the database.
func (r *postgresUserRepository) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error {
	return r.db.WithContext(ctx).Create(rt).Error
}

// GetRefreshTokenByHash retrieves a refresh token using its SHA-256 hex string.
func (r *postgresUserRepository) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&rt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, err
	}
	return &rt, nil
}

// RevokeRefreshToken marks a refresh token as revoked and records the token that replaced it.
func (r *postgresUserRepository) RevokeRefreshToken(ctx context.Context, oldHash string, replacedByHash *string) error {
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("token_hash = ?", oldHash).
		Updates(map[string]interface{}{
			"revoked":                true,
			"replaced_by_token_hash": replacedByHash,
		}).Error
}

// RevokeFamily revokes all refresh tokens belonging to a specific family/session.
// Used defensively when reuse is detected.
func (r *postgresUserRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("family_id = ?", familyID).
		Update("revoked", true).Error
}
