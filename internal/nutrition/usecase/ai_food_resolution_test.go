package usecase

import (
	"testing"

	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
)

func TestAICatalogSearchTerms_PhoPrioritizesCatalogDishNames(t *testing.T) {
	terms := aiCatalogSearchTerms("pho")
	if len(terms) == 0 || terms[0] != "Pho Thin" {
		t.Fatalf("expected Pho Thin first, got %+v", terms)
	}
}

func TestSelectAIFoodCandidate_PrefersMatchingVFADish(t *testing.T) {
	phoID := uuid.New()
	selected := selectAIFoodCandidate("pho", "Pho Thin", []domain.Food{
		{
			ID:              uuid.New(),
			Name:            "Banh pho",
			NameEn:          "Rice noodle",
			Source:          "VFA",
			CaloriesPer100g: 100,
		},
		{
			ID:              phoID,
			Name:            "Pho Thin",
			NameVi:          "Phở Thìn",
			NameEn:          "Pho Thin",
			Source:          "VFA_DISH",
			CaloriesPer100g: 873,
		},
	})

	if selected == nil {
		t.Fatal("expected selected food")
	}
	if selected.ID != phoID {
		t.Fatalf("expected Pho Thin food %s, got %s (%s)", phoID, selected.ID, selected.Name)
	}
}
