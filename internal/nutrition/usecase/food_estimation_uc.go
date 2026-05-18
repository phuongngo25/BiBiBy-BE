package usecase

import (
	"context"
	"fmt"
	"log"

	"nutrix-backend/internal/domain"

	"github.com/google/uuid"
)

type FoodEstimationUseCase struct {
	aiPort   domain.InferencePort
	foodRepo domain.NutritionRepository
}

func NewFoodEstimationUseCase(aiPort domain.InferencePort, repo domain.NutritionRepository) *FoodEstimationUseCase {
	return &FoodEstimationUseCase{
		aiPort:   aiPort,
		foodRepo: repo,
	}
}

// EstimateNutrition xử lý logic lõi: Lấy Volume từ AI -> Lấy Density từ DB -> Tính Mass.
func (uc *FoodEstimationUseCase) EstimateNutrition(ctx context.Context, dishID string, imageBytes []byte) (*domain.Food, error) {
	// 1. Gọi AI qua gRPC Client (InferencePort) để lấy thể tích (cm³)
	volumeCm3, err := uc.aiPort.EstimateVolume(ctx, imageBytes)
	if err != nil {
		return nil, fmt.Errorf("could not estimate volume from image: %w", err)
	}

	// Parse string into UUID
	parsedDishID, err := uuid.Parse(dishID)
	if err != nil {
		return nil, fmt.Errorf("invalid dish ID format: %w", err)
	}

	// 2. Query Database lấy tỷ trọng thức ăn (Density - g/cm³)
	// MOCK: foodRepo.GetDensityByDishID(ctx, dishID)
	// Giả sử lấy được thông tin món ăn từ DB
	foodInfo, err := uc.foodRepo.GetFoodByID(ctx, parsedDishID)
	if err != nil {
		return nil, fmt.Errorf("food item not found: %w", err)
	}

	// Tỷ trọng ví dụ (Density = 1.04 g/cm3)
	// MOCK: Trích xuất density từ foodInfo. Trong thực tế bạn có thể lưu ở bảng riêng.
	density := 1.04

	// 3. Tính toán Mass = Volume (cm³) * Density (g/cm³)
	massGrams := volumeCm3 * density

	log.Printf("[UseCase] Estimated Mass: %.2f grams (Volume: %.2f, Density: %.2f)", massGrams, volumeCm3, density)

	// 4. Tính toán lượng Calories và Macros dựa trên khối lượng
	// Ratio (ví dụ foodInfo lưu lượng chất trên 100g)
	ratio := massGrams / 100.0

	estimatedFood := &domain.Food{
		ID:              foodInfo.ID,
		Name:            foodInfo.Name,
		CaloriesPer100g: foodInfo.CaloriesPer100g * ratio,
		ProteinPer100g:  foodInfo.ProteinPer100g * ratio,
		FatPer100g:      foodInfo.FatPer100g * ratio,
		CarbsPer100g:    foodInfo.CarbsPer100g * ratio,
	}

	return estimatedFood, nil
}
