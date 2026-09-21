# NutriX — API & Integrations Documentation

> **Document Type**: API Reference
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: `internal/nutrition/delivery/http_handler.go`, `internal/user/delivery/http_handler.go`, `internal/workout/delivery/http_handler.go`

---

## API Overview

### Base URL

| Environment | Base URL |
|---|---|
| Production | `https://api.nutrix.app` (via Cloudflared) |
| Development | `http://localhost:8080` |

### Authentication

All protected endpoints require JWT Bearer token:
```
Authorization: Bearer <access_token>
```

Public endpoints (no auth required) are marked below.

---

## Public Endpoints

### Authentication

#### POST `/api/v1/auth/register`

Register a new user account.

**Request Body**:
```json
{
  "username": "john_doe",
  "email": "john@example.com",
  "password": "SecurePass123",
  "full_name": "John Doe",
  "height_cm": 175.0,
  "weight_kg": 70.0,
  "dob": "1990-05-15",
  "gender": "male",
  "activity_level": "active",
  "dietary_preference": "none",
  "medical_conditions": "diabetes_type2,gout"
}
```

**Response** (201 Created):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "uuid-refresh-token",
  "user": {
    "id": "uuid",
    "username": "john_doe",
    "email": "john@example.com",
    "height_cm": 175.0,
    "weight_kg": 70.0,
    "activity_level": "active"
  }
}
```

---

#### POST `/api/v1/auth/login`

Authenticate with email/username and password.

**Request Body**:
```json
{
  "identifier": "john_doe",
  "password": "SecurePass123"
}
```

**Response** (200 OK):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "uuid-refresh-token",
  "user": { ... }
}
```

---

#### POST `/api/v1/auth/refresh`

Exchange refresh token for new access token.

**Request Body**:
```json
{
  "refresh_token": "uuid-refresh-token"
}
```

**Response** (200 OK):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "new-uuid-refresh-token",
  "user": { ... }
}
```

---

### Health Check

#### GET `/ping`

Public health check endpoint.

**Response** (200 OK):
```json
{
  "status": "ok",
  "message": "NutriX is healthy!"
}
```

---

#### GET `/metrics`

Prometheus metrics endpoint.

**Response**: Prometheus text format

---

## Protected Endpoints

All endpoints below require JWT authentication.

---

## User Management

### GET `/api/v1/users/profile`

Get current user's profile.

**Response** (200 OK):
```json
{
  "id": "uuid",
  "username": "john_doe",
  "email": "john@example.com",
  "full_name": "John Doe",
  "height_cm": 175.0,
  "weight_kg": 70.0,
  "dob": "1990-05-15",
  "gender": "male",
  "activity_level": "active",
  "bmr": 1750.0,
  "tdee": 2100.0,
  "goal_type": "maintain",
  "dietary_preference": "none",
  "allergies": "peanut,shellfish",
  "medical_conditions": "diabetes_type2,gout"
}
```

---

### PUT `/api/v1/users/profile`

Update user profile. Recalculates BMR/TDEE if biometrics change.

**Request Body** (all fields optional):
```json
{
  "full_name": "John Updated",
  "height_cm": 180.0,
  "weight_kg": 75.0,
  "date_of_birth": "1990-05-15",
  "gender": "male",
  "activity_level": "very_active",
  "goal_type": "lose_weight",
  "dietary_preference": "vegetarian",
  "allergies": "peanut",
  "medical_conditions": "diabetes_type2"
}
```

---

### GET `/api/v1/users/profile/targets`

Get personalized DRI-based nutrition targets.

**Response** (200 OK):
```json
{
  "total_calories": {
    "current": 850,
    "target": 2000
  },
  "burned_calories": 150,
  "macronutrients": {
    "protein": { "current": 40, "target": 150 },
    "carbs": { "current": 100, "target": 250 },
    "fat": { "current": 30, "target": 67 }
  },
  "micronutrients": [
    { "name": "Sodium", "current": 1500, "target": 2300, "unit": "mg" }
  ]
}
```

---

### GET `/api/v1/users/portfolio`

Get user personalization settings.

**Response** (200 OK):
```json
{
  "user_id": "uuid",
  "preferred_cuisines": ["vietnamese", "thai"],
  "disliked_ingredients": ["cilantro", "mushroom"],
  "excluded_ingredients": ["pork"],
  "meal_schedule": { "breakfast": "08:00", "lunch": "12:00" },
  "daily_water_target_ml": 2500,
  "calorie_target_override": null,
  "macro_split_override": {},
  "notes": ""
}
```

---

### PUT `/api/v1/users/portfolio`

Update user portfolio settings.

**Request Body**:
```json
{
  "preferred_cuisines": ["vietnamese"],
  "disliked_ingredients": ["cilantro"],
  "excluded_ingredients": ["pork", "beef"],
  "daily_water_target_ml": 3000,
  "calorie_target_override": 1800
}
```

---

## Food Management

### GET `/api/v1/nutrition/foods/search`

Search foods with KG safety metadata.

**Query Parameters**:
- `q` (required): Search keyword

**Response** (200 OK):
```json
[
  {
    "id": "uuid",
    "name": "Phở Bò",
    "name_vi": "Phở Bò",
    "name_en": "Beef Pho",
    "category": "Dish",
    "source": "VFA_DISH",
    "calories_per_100g": 90.0,
    "protein_per_100g": 5.0,
    "carbs_per_100g": 13.0,
    "fat_per_100g": 2.0,
    "serving_size": "650g",
    "is_vegan": false,
    "kg_metadata": {
      "is_safe": true,
      "risk_level": "LOW",
      "warnings": []
    }
  }
]
```

---

### GET `/api/v1/nutrition/search-spoonacular`

Search Spoonacular API.

**Query Parameters**:
- `q` (required): Search query
- `diet`: Diet filter (e.g., "vegetarian")
- `intolerances`: Intolerance filter
- `maxCarbs`: Max carbs

**Response**: Same as food search

---

### GET `/api/v1/nutrition/search-by-nutrients`

Search by nutrient ranges.

**Query Parameters**:
- `minProtein`: Minimum protein
- `maxFat`: Maximum fat
- `minCalories`: Minimum calories
- `maxCalories`: Maximum calories

---

### GET `/api/v1/nutrition/search-by-ingredients`

Search by ingredients list.

**Query Parameters**:
- `ingredients` (required): Comma-separated ingredients

---

### POST `/api/v1/nutrition/foods`

Create a custom food entry.

**Request Body**:
```json
{
  "name": "My Protein Bar",
  "category": "Snack",
  "calories_per_100g": 250,
  "protein_per_100g": 20,
  "carbs_per_100g": 30,
  "fat_per_100g": 8,
  "serving_size": "50g",
  "is_vegan": true
}
```

---

### POST `/api/v1/nutrition/foods/upload-image`

Upload food image for CV processing.

**Form Data**:
- `image`: Image file (multipart)

**Response** (200 OK):
```json
{
  "image_url": "/uploads/foods/uuid.jpg"
}
```

---

### POST `/api/v1/nutrition/foods/estimate`

Estimate nutrition from food image.

**Form Data**:
- `image`: Image file (multipart)

**Response** (200 OK):
```json
{
  "food_id": "uuid",
  "food_label": "pho",
  "name": "Phở Bò",
  "calories": 585,
  "protein": 32.5,
  "carbs": 84.5,
  "fat": 13.0,
  "calories_per_100g": 90.0,
  "quantity_grams": 650,
  "confidence": 0.92,
  "source": "cv",
  "estimate_method": "nutrition5k_depth"
}
```

---

### POST `/api/v1/nutrition/foods/scan`

Scan food image and return classification candidates.

**Form Data**:
- `image`: Image file (multipart)

**Response** (200 OK):
```json
{
  "candidates": [
    { "food_label": "pho", "display_name": "Phở", "confidence": 0.92, "mass_g": 650 },
    { "food_label": "bun_bo_hue", "display_name": "Bún Bò Huế", "confidence": 0.05, "mass_g": 700 }
  ]
}
```

---

### GET `/api/v1/nutrition/foods/by-label`

Get foods matching a CV category label.

**Query Parameters**:
- `label` (required): CV label (e.g., "pho")

---

## Meal Logging

### POST `/api/v1/nutrition/log-meal`

Log a food consumption.

**Request Body**:
```json
{
  "food_id": "uuid",
  "quantity_grams": 150,
  "meal_type": "lunch",
  "consumed_date": "2026-08-25",
  "reference_weight_g": 100,
  "acknowledged_risk": false
}
```

**Response** (201 Created):
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "food_id": "uuid",
  "meal_type": "lunch",
  "quantity_grams": 150,
  "calories_consumed": 135,
  "protein_consumed": 7.5,
  "fat_consumed": 3.0,
  "carbs_consumed": 19.5,
  "consumed_date": "2026-08-25",
  "logged_at": "2026-08-25T10:30:00Z"
}
```

**Error Response** (422 Unprocessable Entity):
```json
{
  "error": "meal_blocked",
  "message": "This food conflicts with your allergies",
  "violations": [
    {
      "violation_type": "allergy",
      "description": "Contains peanuts",
      "severity": "CRITICAL"
    }
  ]
}
```

---

### PUT `/api/v1/nutrition/logs/:id`

Update a meal log quantity.

**Request Body**:
```json
{
  "quantity_grams": 200
}
```

---

### POST `/api/v1/nutrition/log-water`

Log water intake.

**Request Body**:
```json
{
  "amount_ml": 250,
  "logged_at": "2026-08-25T10:30:00Z"
}
```

**Response** (201 Created):
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "amount_ml": 250,
  "logged_at": "2026-08-25T10:30:00Z"
}
```

---

## Daily Plan

### GET `/api/v1/nutrition/daily-plan`

Get daily nutrition plan with progress.

**Query Parameters**:
- `date` (optional): Date in YYYY-MM-DD format (default: today)

**Response** (200 OK):
```json
{
  "target_calories": 2000,
  "consumed_calories": 850,
  "burned_calories": 150,
  "target_water": 2000,
  "consumed_water": 750,
  "logged_meals": [
    {
      "id": "uuid",
      "food_id": "uuid",
      "meal_type": "breakfast",
      "quantity_grams": 200,
      "calories_consumed": 400,
      "food": { "name": "Phở Bò", "calories_per_100g": 90 }
    }
  ],
  "recommended_foods": []
}
```

---

## Analytics

### GET `/api/v1/nutrition/analytics/day`

Get daily analytics.

**Query Parameters**:
- `date` (optional): Date in YYYY-MM-DD (default: today)

**Response** (200 OK):
```json
{
  "date": "2026-08-25",
  "consumed_calories": 850,
  "target_calories": 2000,
  "consumed_water": 750,
  "target_water": 2000,
  "estimated_daily_burn": 1850,
  "workout_burned": 150,
  "total_burned": 2000,
  "calorie_goal_hit": false,
  "water_goal_hit": false,
  "goal_hit": false
}
```

---

### GET `/api/v1/nutrition/analytics/weekly`

Get weekly analytics.

**Query Parameters**:
- `days` (optional): Number of days (default: 7)
- `type` (optional): "calendar" for week starting Monday

**Response** (200 OK):
```json
{
  "type": "rolling",
  "window_days": 7,
  "start_date": "2026-08-19",
  "end_date": "2026-08-25",
  "days": [...],
  "weekly_consumed_calories": 4500,
  "weekly_burned_calories": 750,
  "weekly_consumed_water": 10500,
  "streak_days": 5
}
```

---

### GET `/api/v1/nutrition/analytics/monthly`

Get monthly analytics.

**Query Parameters**:
- `month` (optional): Month in YYYY-MM (default: current month)

**Response** (200 OK):
```json
{
  "days": [...],
  "total_consumed_calories": 18000,
  "total_consumed_water": 42000,
  "goal_hit_days": 18,
  "water_goal_hit_days": 15,
  "calorie_goal_hit_days": 20,
  "longest_goal_hit_run": 7,
  "current_goal_hit_run": 3
}
```

---

### GET `/api/v1/nutrition/analytics/streak`

Get user streak information.

**Response** (200 OK):
```json
{
  "current_streak": 5,
  "longest_streak": 14,
  "last_evaluated_date": "2026-08-25"
}
```

---

## AI Features

### GET `/api/v1/nutrition/thresholds`

Get disease-specific nutrient thresholds.

**Response** (200 OK):
```json
{
  "version": 1,
  "generated_at": 1724567890000,
  "not_modified": false,
  "thresholds": [
    { "nutrient_id": "sodium", "warning_mg": 2000, "critical_mg": 3000 },
    { "nutrient_id": "potassium", "warning_mg": 2000, "critical_mg": 4000 }
  ]
}
```

---

### POST `/api/v1/nutrition/meal/validate`

Validate a meal for safety.

**Request Body**:
```json
{
  "candidate": {
    "meal_id": "lunch-1",
    "food_ids": ["uuid1", "uuid2"],
    "meal_type": "lunch",
    "ingredients": ["beef", "noodles"],
    "categories": ["soup", "vietnamese"],
    "protein_sources": ["beef"]
  }
}
```

**Response** (200 OK):
```json
{
  "status": "APPROVED",
  "safe": true,
  "risk_level": "LOW",
  "score": {
    "safety_score": 0.95,
    "macro_score": 0.8,
    "micronutrient_score": 0.7,
    "constraint_score": 0.9
  },
  "violations": [],
  "enrichment": {
    "dish_name": "Phở Bò",
    "estimated_total_weight_g": 650,
    "ingredients": [
      { "name": "Beef", "weight_g": 150 },
      { "name": "Rice Noodles", "weight_g": 300 }
    ],
    "source": "kg",
    "confidence": 0.85
  },
  "fixes": []
}
```

---

### POST `/api/v1/nutrition/recommendations`

Get food recommendations based on nutrient gaps.

**Request Body**:
```json
{
  "gaps": [
    { "nutrient_code": "protein", "gap_amount": 50, "unit": "g" },
    { "nutrient_code": "iron", "gap_amount": 5, "unit": "mg" }
  ]
}
```

**Response** (200 OK):
```json
{
  "recommendations": [
    {
      "food_id": "uuid",
      "food_name": "Beef Steak",
      "match_score": 0.92,
      "reasons": [
        { "nutrient_code": "protein", "contribution_score": 0.8, "reason_type": "MACRO_MATCH" }
      ]
    }
  ]
}
```

---

### POST `/api/v1/nutrition/feedback/correction`

Submit food prediction correction.

**Request Body**:
```json
{
  "request_id": "uuid",
  "predicted_food_name": "Phở",
  "final_food_name": "Bún Bò Huế",
  "prediction_confidence": 0.7,
  "image_hash": "sha256..."
}
```

---

## Planner

### POST `/api/v1/planner/weekly-plan`

Generate a 7-day meal plan.

**Request Body**:
```json
{
  "startDate": "2026-08-26",
  "goal": "lose_weight",
  "goalType": "lose_weight",
  "strategy": "balanced",
  "candidate_foods": []
}
```

**Response** (200 OK):
```json
{
  "plan_id": "uuid",
  "meals": [
    {
      "meal_id": "mon-breakfast",
      "date": "2026-08-26",
      "food_ids": ["uuid"],
      "meal_type": "breakfast",
      "status": "planned",
      "food_name": "Phở Bò",
      "calories": 585
    }
  ]
}
```

---

### POST `/api/v1/planner/reoptimize`

Reoptimize plan with a meal swap.

**Request Body**:
```json
{
  "planId": "uuid",
  "adjustment": {
    "type": "swap_meal",
    "targetDate": "2026-08-26",
    "targetMealType": "lunch"
  }
}
```

---

### POST `/api/v1/planner/explain`

Get reasoning for plan decisions.

**Request Body**: Same as `/nutrition/meal/validate`

---

## Gamification

### GET `/api/v1/gamification/achievements`

Get all achievements with user's unlock status.

**Response** (200 OK):
```json
{
  "definitions": [
    { "id": "first_goal_hit", "title": "First Victory", "category": "milestone" },
    { "id": "streak_7", "title": "Week Warrior", "category": "streak" },
    { "id": "streak_30", "title": "Monthly Master", "category": "streak" }
  ],
  "unlocked": [
    { "achievement_id": "first_goal_hit", "unlocked_at": "2026-08-01T00:00:00Z" }
  ]
}
```

---

## Workouts

### GET `/api/v1/exercises`

Search exercises by body parts.

**Query Parameters**:
- `bodyParts` (required): Comma-separated body parts

---

### GET `/api/v1/exercises/:id`

Get exercise details.

---

### POST `/api/v1/workouts/log`

Log a workout session.

**Request Body**:
```json
{
  "exercise_id": "exercise-123",
  "exercise_name": "Running",
  "duration_minutes": 30,
  "met_value": 9.8
}
```

---

## OpenFoodFacts Proxy

### GET `/api/v1/off-proxy/product`

Proxy to OpenFoodFacts API.

**Query Parameters**:
- `barcode` (required): Product barcode

---

## Error Responses

| HTTP Status | Meaning | Response Format |
|---|---|---|
| 400 | Bad Request | `{ "error": "validation message" }` |
| 401 | Unauthorized | `{ "error": "invalid token" }` |
| 404 | Not Found | `{ "error": "resource not found" }` |
| 409 | Conflict | `{ "error": "duplicate resource" }` |
| 422 | Unprocessable Entity | `{ "error": "meal_blocked", "violations": [...] }` |
| 500 | Internal Error | `{ "error": "internal server error" }` |

---

## Rate Limiting

| Endpoint Type | Limit |
|---|---|
| Public (auth) | 20 requests/minute |
| Protected | 150 requests/minute |

**Response Headers**:
- `X-RateLimit-Limit`: Maximum requests
- `X-RateLimit-Remaining`: Remaining requests
- `X-RateLimit-Reset`: Unix timestamp reset

---

## External Integrations

### Spoonacular API

**Purpose**: Food search fallback
**Config**: `SPOONACULAR_API_KEY`
**Rate Limits**: Per Spoonacular tier

### RapidAPI ExerciseDB

**Purpose**: Exercise catalog
**Config**: `RAPIDAPI_KEY`
**Rate Limits**: Per RapidAPI tier

### OpenFoodFacts

**Purpose**: Product barcode lookup
**Integration**: Same-origin proxy
**UA Required**: `NutriX/1.0 (https://bibiby.space)`

### AI Services (gRPC)

| Service | Host | Port | Purpose |
|---|---|---|---|
| ai-kg | GRPC_AI_HOST | 50051 | Knowledge Graph |
| ai-cv | GRPC_CV_HOST | 50052 | Computer Vision |

---

## Files

- Evidence: `internal/nutrition/delivery/http_handler.go`
- Evidence: `internal/user/delivery/http_handler.go`
- Evidence: `internal/workout/delivery/http_handler.go`
