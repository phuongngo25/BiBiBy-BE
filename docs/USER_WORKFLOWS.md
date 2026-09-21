# NutriX — User Workflows

> **Document Type**: User Journey Documentation
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: `internal/nutrition/delivery/http_handler.go`, `internal/user/delivery/http_handler.go`, `internal/nutrition/usecase/nutrition_usecase.go`

---

## Major User Journeys

### Journey 1: New User Onboarding

```
Actor: New user
Trigger: First app launch
Preconditions: None
```

#### Steps

1. **Registration**
   - User submits registration form (username, email, password)
   - Optional: Height, weight, DOB, gender, activity level, dietary preference, medical conditions
   - Backend creates user record and JWT tokens

2. **Profile Completion**
   - User prompted to enter biometrics (height, weight, DOB)
   - Backend calculates BMR using Mifflin-St Jeor equation
   - Backend calculates TDEE based on activity level
   - Backend generates initial calorie/water targets

3. **First Food Search**
   - User searches for "pho"
   - Backend searches local DB → returns VFA dish entry
   - KG metadata attached with safety signals

4. **First Meal Log**
   - User selects food, enters quantity (150g)
   - Backend validates against user profile (allergies, conditions)
   - If conflict → user shown warning with option to acknowledge
   - Meal logged, streak evaluated

#### System Behavior

- `POST /api/v1/auth/register` → creates user + returns JWT
- `PUT /api/v1/users/profile` → updates biometrics → recalculates BMR/TDEE
- `GET /api/v1/nutrition/foods/search?q=pho` → returns foods with KG metadata
- `POST /api/v1/nutrition/log-meal` → validates + logs meal → evaluates streak

#### Failure Cases

| Failure | Handling |
|---|---|
| Duplicate email/username | 409 Conflict error |
| Weak password | 400 validation error |
| Food not found | Fallback to Spoonacular |
| All foods rejected by KG | 422 with detailed violations |
| Profile conflict | Warning shown, user can acknowledge |

---

### Journey 2: Daily Meal Logging

```
Actor: Existing user
Trigger: User opens app to log meal
Preconditions: User is authenticated
```

#### Steps

1. **Search Food**
   - User types food name in search bar
   - Backend performs trigram search on local DB
   - Results enriched with KG safety metadata

2. **Select Food**
   - User taps on food item
   - Nutrition info displayed (calories, protein, carbs, fat)

3. **Enter Quantity**
   - User enters portion size in grams
   - Backend scales macros proportionally

4. **Select Meal Type**
   - User selects: Breakfast, Lunch, Dinner, Snack
   - User selects date (defaults to today)

5. **Submit Log**
   - Backend validates against profile
   - If allergies/conditions conflict → warning displayed
   - User can acknowledge or cancel
   - Meal logged → daily plan updated

6. **View Progress**
   - Dashboard shows: consumed vs target calories
   - Water tracking shown separately
   - Remaining calories/macros calculated

#### System Behavior

```
GET /api/v1/nutrition/foods/search?q=bread
  ↓
POST /api/v1/nutrition/log-meal
  ↓
GET /api/v1/nutrition/daily-plan?date=2026-08-25
```

#### Improved Workflow (With CV)

1. User opens camera
2. Takes photo of meal
3. CV recognizes dish → top-K candidates returned
4. User selects correct dish from candidates
5. AI estimates portion weight from depth data
6. User confirms or adjusts
7. One-tap log → profile validation → success

---

### Journey 3: Photo-Based Food Recognition

```
Actor: User with meal photo
Trigger: User takes photo of food
Preconditions: User has image of meal
```

#### Steps

1. **Capture Image**
   - User takes photo of meal
   - Image uploaded to backend via multipart/form-data

2. **Dish Classification**
   - CV model analyzes image
   - Returns top-K dish candidates with confidence scores

3. **Food Resolution**
   - User selects from candidates OR
   - User enters food name manually
   - Backend resolves to database entry

4. **Portion Estimation** [PARTIAL]
   - AI estimates mass from depth-aware analysis
   - User confirms or adjusts quantity

5. **Log Confirmation**
   - Same as Journey 2, Step 4-6

#### Current Limitations

- CV accuracy not validated in production
- Depth estimation requires special camera setup
- MinIO storage for images not fully integrated

#### System Behavior

```
POST /api/v1/nutrition/foods/upload-image
  ↓
POST /api/v1/nutrition/foods/estimate  (or /scan for candidates)
  ↓
POST /api/v1/nutrition/log-meal
```

---

### Journey 4: Food Safety Analysis

```
Actor: User with medical condition
Trigger: User views food safety info
Preconditions: User has configured medical conditions in profile
```

#### Steps

1. **View Food**
   - User searches for food item
   - Food card shows basic nutrition + KG safety badge

2. **Deep Safety Analysis**
   - User taps "Analyze Safety"
   - Backend calls KG `AnalyzeFood`
   - Returns: safe/warning/rejected status + violations

3. **Explanation**
   - User taps "Why?"
   - KG returns reasoning path (if available)
   - Shows disease-specific warnings

4. **Decision**
   - Safe → User can log confidently
   - Warning → User shown risk details
   - Rejected → User blocked unless acknowledged

#### Current Gap

- Disease context not fully wired to all KG calls
- Only allergies are locally validated
- Medical conditions passed as empty list to KG

#### System Behavior

```
GET /api/v1/nutrition/foods/search?q=salt
  ↓
GET /api/v1/nutrition/thresholds
  ↓
POST /api/v1/nutrition/meal/validate
```

---

### Journey 5: Weekly Meal Planning

```
Actor: User seeking meal plan
Trigger: User requests weekly plan
Preconditions: User has complete profile
```

#### Steps

1. **Request Plan**
   - User selects start date and goal
   - Optionally specifies food preferences
   - Backend generates 7-day plan

2. **Plan Generation**
   - Backend loads candidate foods from DB
   - Filters by user profile (allergies, diet, exclusions)
   - Validates each meal via KG
   - Assembles into weekly schedule

3. **Review Plan**
   - User views 7-day breakdown
   - Each meal shows: food name, calories, protein
   - Safety badges on each meal

4. **Adjust Plan**
   - User can swap individual meals
   - Backend re-validates replacement
   - Plan updated in real-time

5. **Execute Plan**
   - User logs meals as they eat
   - Daily progress tracked
   - Deviation from plan noted

#### Current Limitations

- Plan cached in-memory (lost on restart)
- No horizontal scaling (single instance only)
- Limited candidate food pool (100 items max)

#### System Behavior

```
POST /api/v1/planner/weekly-plan
  ↓
GET /api/v1/planner/reoptimize (for swaps)
  ↓
POST /api/v1/planner/explain (for reasoning)
```

---

### Journey 6: Analytics Review

```
Actor: User reviewing progress
Trigger: User opens analytics dashboard
Preconditions: User has logged meals
```

#### Steps

1. **View Daily Summary**
   - Dashboard shows today's progress
   - Calories: consumed vs target
   - Water: consumed vs target
   - Workouts: burned calories

2. **Weekly Overview**
   - 7-day rolling view OR calendar week
   - Total calories consumed
   - Total burned (workout + base)
   - Streak count

3. **Monthly Deep Dive**
   - 30-day view
   - Goal hit days
   - Longest streak
   - Macro distribution

4. **Streak Analysis**
   - Current streak
   - Longest streak
   - Achievements unlocked

#### System Behavior

```
GET /api/v1/nutrition/analytics/day?date=2026-08-25
GET /api/v1/nutrition/analytics/weekly
GET /api/v1/nutrition/analytics/monthly?month=2026-08
GET /api/v1/nutrition/analytics/streak
GET /api/v1/gamification/achievements
```

---

## Repetitive Actions Removed

| Before | After | Improvement |
|---|---|---|
| Type full food name | Photo capture + CV | 80% less typing |
| Look up calories | Auto-populated from DB | Zero lookup |
| Calculate portion math | Server-side calculation | No mental math |
| Check food safety manually | KG analysis on search | Real-time safety |

---

## Steps Automated

| Automation | Evidence |
|---|---|
| BMR/TDEE calculation | `HealthCalculationService` |
| Daily target calculation | `GoalStrategy` |
| Streak evaluation | `evaluateStreakHook` after meal log |
| Achievement checking | `GamificationService` after streak eval |
| KG metadata enrichment | Background Redis caching |
| Food database fallback | Spoonacular proxy |

---

## Human Decisions Preserved

| Decision | Why Preserved | Where |
|---|---|---|
| Food selection | User preference | Client UI |
| Portion confirmation | Accuracy | Client UI |
| Risk acknowledgment | Medical responsibility | Client UI + Server validation |
| Meal plan acceptance | Lifestyle fit | Client UI |
| Achievement celebration | Motivation | Client UI |

---

## Time Savings (Estimated)

| Task | Before | After | Evidence |
|---|---|---|---|
| Single food log | 3-5 min | 30 sec | CV + auto-search |
| Weekly planning | 1-2 hours | 5 min | AI plan generation |
| Safety research | 10-30 min | Real-time | KG analysis |
| Daily review | 15 min | 1 min | Dashboard aggregation |

*Note: CV accuracy and AI plan quality not yet validated*
