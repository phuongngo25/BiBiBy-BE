package service

import (
	"testing"
	"time"

	"nutrix-backend/internal/domain"
)

func TestHealthCalculationService_BMRAndTDEE(t *testing.T) {
	calcService := NewHealthCalculationService()

	// Setup a typical DOB for ~30 year old user
	now := time.Now()
	dobMale := now.AddDate(-30, 0, 0) // exact 30 years ago
	dobFemale := now.AddDate(-25, 0, 0) // exact 25 years ago

	tests := []struct {
		name          string
		weight        float64
		height        float64
		dob           time.Time
		gender        string
		activity      domain.ActivityLevel
		expectedBMR   int
		expectedTDEE  int
	}{
		{
			name:          "Male, Active, 30yo, 80kg, 180cm",
			weight:        80,
			height:        180,
			dob:           dobMale,
			gender:        "male",
			activity:      domain.ActivityActive,
			expectedBMR:   1785, // 10*80 + 6.25*180 - 5*29 + 5 = 1785 -> rounded 1785
			expectedTDEE:  2767, // 1785 * 1.55 = 2766.75 -> rounded 2767
		},
		{
			name:          "Female, Sedentary, 25yo, 60kg, 165cm",
			weight:        60,
			height:        165,
			dob:           dobFemale,
			gender:        "female",
			activity:      domain.ActivitySedentary,
			expectedBMR:   1345, // 10*60 + 6.25*165 - 5*25 - 161 = 1345.25 -> rounded 1345
			expectedTDEE:  1614, // 1345 * 1.2 = 1614 -> rounded 1614
		},
		{
			name:          "Invalid Weight or Height",
			weight:        0,
			height:        180,
			dob:           dobMale,
			gender:        "male",
			activity:      domain.ActivityActive,
			expectedBMR:   0,
			expectedTDEE:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bmr := calcService.CalculateBMR(tt.weight, tt.height, tt.dob, tt.gender)
			if bmr != tt.expectedBMR {
				t.Errorf("expected BMR %d, got %d", tt.expectedBMR, bmr)
			}

			tdee := calcService.CalculateTDEE(bmr, tt.activity)
			if tdee != tt.expectedTDEE {
				t.Errorf("expected TDEE %d, got %d", tt.expectedTDEE, tdee)
			}
		})
	}
}

func TestGoalStrategyV1_CalculateTargets(t *testing.T) {
	strategy := NewGoalStrategyV1()

	tests := []struct {
		name           string
		user           *domain.User
		expectedCal    int
		expectedWater  int
	}{
		{
			name: "Calorie shift Lose Weight",
			user: &domain.User{
				Gender:        "male",
				WeightKg:      80,
				ActivityLevel: domain.ActivityActive,
				TDEE:          2500,
				GoalType:      domain.GoalLoseWeight,
			},
			expectedCal:   2000, // 2500 - 500
			expectedWater: 3350, // Base: 80 * 30 = 2400 + 250 (male) + 700 (active) = 3350
		},
		{
			name: "Calorie shift Gain Weight",
			user: &domain.User{
				Gender:        "female",
				WeightKg:      60,
				ActivityLevel: domain.ActivityLowActive,
				TDEE:          1800,
				GoalType:      domain.GoalGainWeight,
			},
			expectedCal:   2300, // 1800 + 500
			expectedWater: 2150, // Base: 60 * 30 = 1800 + 0 (female) + 350 (low_active) = 2150
		},
		{
			name: "Manual Calorie Override",
			user: &domain.User{
				Gender:              "male",
				WeightKg:            70,
				ActivityLevel:       domain.ActivitySedentary,
				TDEE:                2000,
				GoalType:            domain.GoalLoseWeight,
				WeeklyCalorieBudget: 14000, // 14000 / 7 = 2000 per day (overrides LoseWeight shift)
			},
			expectedCal:   2000,
			expectedWater: 2350, // Base: 70 * 30 = 2100 + 250 (male) + 0 (sedentary) = 2350
		},
		{
			name: "Calorie under healthy floor (clamped to 1200)",
			user: &domain.User{
				Gender:        "female",
				WeightKg:      45,
				ActivityLevel: domain.ActivitySedentary,
				TDEE:          1400,
				GoalType:      domain.GoalLoseWeight, // 1400 - 500 = 900
			},
			expectedCal:   1200, // clamped to 1200
			expectedWater: 1500, // Base: 45 * 30 = 1350. Clamped to 1500 min.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calories, water := strategy.CalculateTargets(tt.user)
			if calories != tt.expectedCal {
				t.Errorf("expected Calories %d, got %d", tt.expectedCal, calories)
			}
			if water != tt.expectedWater {
				t.Errorf("expected Water %d, got %d", tt.expectedWater, water)
			}
		})
	}
}
