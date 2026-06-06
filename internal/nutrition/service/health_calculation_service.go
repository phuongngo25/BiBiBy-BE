package service

import (
	"math"
	"strings"
	"time"

	"nutrix-backend/internal/domain"
)

// HealthCalculationService handles physiological health calculations like BMR and TDEE.
type HealthCalculationService interface {
	CalculateBMR(weightKg, heightCm float64, dob time.Time, gender string) int
	CalculateTDEE(bmr int, activityLevel domain.ActivityLevel) int
}

type healthCalculationService struct{}

// NewHealthCalculationService creates a new instance of HealthCalculationService.
func NewHealthCalculationService() HealthCalculationService {
	return &healthCalculationService{}
}

// CalculateBMR calculates the Basal Metabolic Rate using the Harris-Benedict formula.
func (s *healthCalculationService) CalculateBMR(weightKg, heightCm float64, dob time.Time, gender string) int {
	if weightKg <= 0 || heightCm <= 0 {
		return 0
	}

	// Calculate age
	now := time.Now()
	age := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		age--
	}
	if age <= 0 || age > 120 {
		age = 25 // standard fallback age if dob is empty or invalid
	}

	var bmr float64
	if strings.ToLower(gender) == "female" || strings.ToLower(gender) == "females" {
		bmr = (10 * weightKg) + (6.25 * heightCm) - (5 * float64(age)) - 161
	} else {
		bmr = (10 * weightKg) + (6.25 * heightCm) - (5 * float64(age)) + 5
	}

	return int(math.Round(bmr))
}

// CalculateTDEE calculates the Total Daily Energy Expenditure using the activity multiplier.
func (s *healthCalculationService) CalculateTDEE(bmr int, activityLevel domain.ActivityLevel) int {
	if bmr <= 0 {
		return 0
	}

	var multiplier float64
	switch activityLevel {
	case domain.ActivitySedentary:
		multiplier = 1.2
	case domain.ActivityLowActive:
		multiplier = 1.375
	case domain.ActivityActive:
		multiplier = 1.55
	case domain.ActivityVeryActive:
		multiplier = 1.725
	default:
		multiplier = 1.2
	}

	return int(math.Round(float64(bmr) * multiplier))
}
