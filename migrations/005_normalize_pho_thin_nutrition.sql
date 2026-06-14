-- VFA_DISH stores these Phở Thìn nutrients per complete dish, not per 100 g.
-- Existing enrichment evidence defines one complete serving as 650 g.
UPDATE foods
SET
    calories_per_100g = 873.0 / 6.5,
    protein_per_100g = 42.0 / 6.5,
    fat_per_100g = 41.4 / 6.5,
    carbs_per_100g = 83.1 / 6.5,
    serving_size = '650g'
WHERE code IN ('vfa_dish_HAN-112002', 'vfa_dish_SFF-112002')
  AND source = 'VFA_DISH';
