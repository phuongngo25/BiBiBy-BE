-- Explicit flag replacing the client-side "servingQuantity != 100" heuristic
-- used to decide whether a food's CaloriesPer100g/ProteinPer100g/CarbsPer100g/
-- FatPer100g needs the app's runtime correction (see meal_detail_bloc.dart).
--
-- Default true: USDA and custom/user-entered foods already provide genuine
-- per-100g nutrition data. Two sources are NOT genuinely per-100g:
--   - VFA_DISH-seeded dishes: raw data is whole-dish totals with no known
--     portion size (see migration 005's comment).
--   - Spoonacular-sourced foods (mapComplexResultsToFoods /
--     mapNutrientResultsToFoods): confirmed via Spoonacular's own API docs
--     ("Nutrition data is per serving") that GetNutrient("Calories")/.Calories
--     is per-serving, not per-100g, and the API provides no serving weight in
--     grams to convert with.
-- Both are presumed un-normalized below.
--
-- The two VFA_DISH seeding code paths in this repo
-- (internal/nutrition/seeder/seeder.go and pkg/database/postgres.go) diverged
-- and prefix dish codes differently ("DISH-HAN-112002" vs
-- "vfa_dish_HAN-112002"), so both are matched below.
ALTER TABLE foods ADD COLUMN IF NOT EXISTS is_portion_normalized BOOLEAN NOT NULL DEFAULT true;

UPDATE foods
SET is_portion_normalized = false
WHERE source = 'VFA_DISH';

UPDATE foods
SET is_portion_normalized = true
WHERE source = 'VFA_DISH'
  AND (code ILIKE '%HAN-112002' OR code ILIKE '%SFF-112002');

UPDATE foods
SET is_portion_normalized = false
WHERE source = 'Spoonacular';
