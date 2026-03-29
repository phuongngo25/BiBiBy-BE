package domain

import (
	"time"

	"github.com/google/uuid"
)

// MealPlan represents a scheduled daily goal for the user.
type MealPlan struct {
	ID             uuid.UUID      `json:"id"              gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID         uuid.UUID      `json:"user_id"         gorm:"index;not null"`
	PlanDate       time.Time      `json:"plan_date"       gorm:"type:date;not null"`
	TargetCalories float64        `json:"target_calories" gorm:"not null"`
	Status         string         `json:"status"          gorm:"default:'planned'"`
	CreatedAt      time.Time      `json:"created_at"`
	
	Items          []MealPlanItem `json:"items"           gorm:"foreignKey:MealPlanID"`
}

// MealPlanItem represents a specific food item planned for a MealPlan.
type MealPlanItem struct {
	ID                   uuid.UUID `json:"id"                     gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	MealPlanID           uuid.UUID `json:"meal_plan_id"           gorm:"index;not null;constraint:OnDelete:CASCADE;"`
	FoodID               uuid.UUID `json:"food_id"                gorm:"index;not null"`
	MealType             string    `json:"meal_type"`
	PlannedQuantityGrams float64   `json:"planned_quantity_grams" gorm:"not null"`

	Food                 Food      `json:"food"                   gorm:"foreignKey:FoodID"`
}
