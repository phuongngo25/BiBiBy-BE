package seeder

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nutrix-backend/internal/domain"
)

// -------------------------------------------------------------------------
// DTO Structs for Streaming JSON Parsing
// -------------------------------------------------------------------------

// USDA Core Foods (usda_core_foods.json)
type USDAFoodNutrientDTO struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	UnitName string  `json:"unitName"`
}

type USDAFoodDTO struct {
	FDCId         int                   `json:"fdcId"`
	Description   string                `json:"description"`
	DataType      string                `json:"dataType"`
	FoodNutrients []USDAFoodNutrientDTO `json:"foodNutrients"`
}

// VFA Food DB (vfa_food_db.json)
type VFANutrientDTO struct {
	NameEn string  `json:"name_en"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
}

type VFAFoodDTO struct {
	Code       string           `json:"code"`
	NameVi     string           `json:"name_vi"`
	NameEn     string           `json:"name_en"`
	CategoryEn string           `json:"categoryEn"`
	Category   string           `json:"category"`
	Nutrition  []VFANutrientDTO `json:"nutrition"`
}

// VFA Dishes DB (vfa_dishes_db.json)
type VFADishNutrientDTO struct {
	NameEn string `json:"nameEn"`
	Amount any    `json:"amount"`
	Unit   string `json:"unit_name"`
}

type VFADishDTO struct {
	Code                  string               `json:"code"`
	NameVi                string               `json:"name_vi"`
	NameEn                string               `json:"name_en"`
	NutritionalComponents []VFADishNutrientDTO `json:"nutritional_components"`
}

const batchSize = 500

// SeedBaseTruthData is the entrypoint to auto-seed base truth foods into PostgreSQL using a Streaming Decoder footprint.
func SeedBaseTruthData(db *gorm.DB) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get cwd: %w", err)
	}

	log.Println("[Seeder] Generating VFA Foods...")
	if err := seedVFAFood(db, filepath.Join(cwd, "vfa_food_db.json")); err != nil {
		log.Printf("WARNING: Failed to seed VFA Foods -> %v\n", err)
	}

	log.Println("[Seeder] Generating VFA Dishes...")
	if err := seedVFADish(db, filepath.Join(cwd, "vfa_dishes_db.json")); err != nil {
		log.Printf("WARNING: Failed to seed VFA Dishes -> %v\n", err)
	}

	log.Println("[Seeder] Generating USDA Foods... (This may take a moment)")
	if err := seedUSDA(db, filepath.Join(cwd, "usda_core_foods.json")); err != nil {
		log.Printf("WARNING: Failed to seed USDA Foods -> %v\n", err)
	}

	log.Println("[Seeder] Generating Dietary Reference Intakes (DRIs)...")
	if err := seedDRIs(db, filepath.Join(cwd, "DRIs.json")); err != nil {
		log.Printf("WARNING: Failed to seed DRIs -> %v\n", err)
	}

	log.Println("[Seeder] Batch UPSERT complete!")
	return nil
}

// -------------------------------------------------------------------------
// DRI Parsing Logic
// -------------------------------------------------------------------------

func seedDRIs(db *gorm.DB, path string) error {
	var count int64
	db.Model(&domain.DRI{}).Count(&count)
	if count > 0 {
		return nil // skip if already seeded
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file error: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := assertJsonArray(decoder); err != nil {
		return err
	}

	var batch []domain.DRI
	for decoder.More() {
		var dto domain.DRI
		if err := decoder.Decode(&dto); err != nil {
			return fmt.Errorf("decode error: %w", err)
		}
		batch = append(batch, dto)
	}
	
	if len(batch) > 0 {
		return db.Create(&batch).Error
	}
	return nil
}

// -------------------------------------------------------------------------
// VFA Food Parsing Logic
// -------------------------------------------------------------------------

func seedVFAFood(db *gorm.DB, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file error: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := assertJsonArray(decoder); err != nil {
		return err
	}

	var batch []domain.Food
	for decoder.More() {
		var dto VFAFoodDTO
		if err := decoder.Decode(&dto); err != nil {
			return fmt.Errorf("decode error: %w", err)
		}

		cat := dto.CategoryEn
		if cat == "" {
			cat = dto.Category
		}

		food := domain.Food{
			Code:           "VFA-" + dto.Code,
			Name:           dto.NameEn,
			NameEn:         dto.NameEn,
			NameVi:         dto.NameVi,
			Category:       cat,
			Source:         "VFA",
			IsVerified:     true,
			Micronutrients: make(datatypes.JSONMap),
		}

		if food.Name == "" {
			food.Name = food.NameVi
		}

		for _, nut := range dto.Nutrition {
			nName := strings.ToLower(nut.NameEn)
			switch {
			case strings.Contains(nName, "energy"):
				food.CaloriesPer100g = nut.Value
			case nName == "protein":
				food.ProteinPer100g = nut.Value
			case strings.Contains(nName, "lipid") || strings.Contains(nName, "fat"):
				food.FatPer100g = nut.Value
			case strings.Contains(nName, "carbohydrate"):
				food.CarbsPer100g = nut.Value
			default:
				food.Micronutrients[nut.NameEn] = fmt.Sprintf("%f %s", nut.Value, nut.Unit)
			}
		}

		batch = append(batch, food)
		if len(batch) >= batchSize {
			flushBatch(db, batch)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		flushBatch(db, batch)
	}
	return nil
}

// -------------------------------------------------------------------------
// VFA Dish Parsing Logic
// -------------------------------------------------------------------------

func seedVFADish(db *gorm.DB, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file error: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := assertJsonArray(decoder); err != nil {
		return err
	}

	var batch []domain.Food
	for decoder.More() {
		var dto VFADishDTO
		if err := decoder.Decode(&dto); err != nil {
			return fmt.Errorf("decode error: %w", err)
		}

		food := domain.Food{
			Code:           "DISH-" + dto.Code,
			Name:           dto.NameEn,
			NameEn:         dto.NameEn,
			NameVi:         dto.NameVi,
			Category:       "Prepared Dish",
			Source:         "VFA_DISH",
			IsVerified:     true,
			Micronutrients: make(datatypes.JSONMap),
		}

		if food.Name == "" {
			food.Name = food.NameVi
		}

		for _, nut := range dto.NutritionalComponents {
			nName := strings.ToLower(nut.NameEn)
			
			var amount float64
			switch val := nut.Amount.(type) {
			case float64:
				amount = val
			case string:
				parsed, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
				amount = parsed
			}

			switch {
			case strings.Contains(nName, "energy"):
				food.CaloriesPer100g = amount
			case nName == "protein":
				food.ProteinPer100g = amount
			case strings.Contains(nName, "lipid") || strings.Contains(nName, "fat"):
				food.FatPer100g = amount
			case strings.Contains(nName, "carbohydrate"):
				food.CarbsPer100g = amount
			default:
				food.Micronutrients[nut.NameEn] = fmt.Sprintf("%f %s", amount, nut.Unit)
			}
		}

		batch = append(batch, food)
		if len(batch) >= batchSize {
			flushBatch(db, batch)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		flushBatch(db, batch)
	}
	return nil
}

// -------------------------------------------------------------------------
// USDA Parsing Logic
// -------------------------------------------------------------------------

func seedUSDA(db *gorm.DB, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file error: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := assertJsonArray(decoder); err != nil {
		return err
	}

	var batch []domain.Food
	for decoder.More() {
		var dto USDAFoodDTO
		if err := decoder.Decode(&dto); err != nil {
			return fmt.Errorf("decode error: %w", err)
		}

		food := domain.Food{
			Code:           fmt.Sprintf("USDA-%d", dto.FDCId),
			Name:           dto.Description,
			NameEn:         dto.Description,
			Category:       dto.DataType,
			Source:         "USDA",
			IsVerified:     true,
			Micronutrients: make(datatypes.JSONMap),
		}

		for _, nut := range dto.FoodNutrients {
			nName := strings.ToLower(nut.Name)
			switch {
			case strings.Contains(nName, "energy"):
				if food.CaloriesPer100g == 0 && strings.Contains(strings.ToLower(nut.UnitName), "kcal") {
					food.CaloriesPer100g = nut.Amount
				} // prioritize kcal over kJ
			case nName == "protein":
				food.ProteinPer100g = nut.Amount
			case strings.Contains(nName, "lipid") || strings.Contains(nName, "fat"):
				food.FatPer100g = nut.Amount
			case strings.Contains(nName, "carbohydrate"):
				food.CarbsPer100g = nut.Amount
			default:
				food.Micronutrients[nut.Name] = fmt.Sprintf("%f %s", nut.Amount, nut.UnitName)
			}
		}

		batch = append(batch, food)
		if len(batch) >= batchSize {
			flushBatch(db, batch)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		flushBatch(db, batch)
	}
	return nil
}

// -------------------------------------------------------------------------
// DB and Decoder Helpers
// -------------------------------------------------------------------------

func flushBatch(db *gorm.DB, foods []domain.Food) {
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"name_en",
			"name_vi",
			"category",
			"calories_per_100g",
			"protein_per_100g",
			"carbs_per_100g",
			"fat_per_100g",
			"micronutrients",
			"source",
		}),
	}).CreateInBatches(foods, len(foods)).Error

	if err != nil {
		log.Printf("Batch insert failure: %v", err)
	}
}

func assertJsonArray(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("expected JSON array start '['")
	}
	return nil
}
