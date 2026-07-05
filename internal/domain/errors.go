package domain

import "errors"

// Sentinel errors shared across all domain packages.
var (
	ErrFoodNotFound        = errors.New("food not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInternalServerError = errors.New("internal server error")
	ErrDRINotFound         = errors.New("no DRI record found for this demographic — please complete your profile")
	ErrProfileIncomplete   = errors.New("profile incomplete: date of birth and gender are required to calculate targets")
	ErrWeakPassword        = errors.New("password must be at least 8 characters long, contain at least 1 uppercase letter, 1 lowercase letter, and 1 number")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	ErrReuseDetected       = errors.New("refresh token reuse detected")
	ErrLogNotFound         = errors.New("food log entry not found")
	ErrInvalidQuantity     = errors.New("invalid quantity: must be between 1 and 5000 grams")
	ErrAllCandidatesRejected = errors.New("all candidate meals were rejected due to safety violations")
	ErrMealBlockedByProfile  = errors.New("this food conflicts with your health profile")
)

// MealBlockedError is returned by LogMeal when a food conflicts with the user's
// health profile (allergy/disease/diet) and the request has not explicitly
// acknowledged the risk. It carries the offending violations so the API layer
// can surface them to the client for a confirm-to-override prompt.
type MealBlockedError struct {
	Violations []MealViolation
}

func (e *MealBlockedError) Error() string { return ErrMealBlockedByProfile.Error() }

// Unwrap lets errors.Is(err, ErrMealBlockedByProfile) match.
func (e *MealBlockedError) Unwrap() error { return ErrMealBlockedByProfile }
