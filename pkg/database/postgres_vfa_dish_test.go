package database

import (
	"math"
	"testing"

	"nutrix-backend/internal/domain"
)

func TestNormalizeKnownVFADishServing(t *testing.T) {
	food := domain.Food{
		CaloriesPer100g: 873,
		ProteinPer100g:  42,
		FatPer100g:      41.4,
		CarbsPer100g:    83.1,
		ServingSize:     "100g",
	}

	normalizeKnownVFADishServing(&food, "SFF-112002")

	assertNear := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.001 {
			t.Fatalf("%s: got %.4f, want %.4f", name, got, want)
		}
	}

	assertNear("calories", food.CaloriesPer100g, 873.0/6.5)
	assertNear("protein", food.ProteinPer100g, 42.0/6.5)
	assertNear("fat", food.FatPer100g, 41.4/6.5)
	assertNear("carbs", food.CarbsPer100g, 83.1/6.5)
	if food.ServingSize != "650g" {
		t.Fatalf("serving size: got %q, want 650g", food.ServingSize)
	}
}
