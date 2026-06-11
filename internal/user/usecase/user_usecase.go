package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/service"
	"nutrix-backend/pkg/crypto"
)

type userUseCase struct {
	repo               domain.UserRepository
	portfolioRepo      domain.UserPortfolioRepository
	driRepo            domain.DRIRepository
	nutriRepo          domain.NutritionRepository
	workoutRepo        domain.WorkoutRepository
	jwtSecret          string
	jwtExpirationHours int
}

// NewUserUseCase creates a UserUseCase with the provided repository and JWT config.
func NewUserUseCase(repo domain.UserRepository, driRepo domain.DRIRepository, nutriRepo domain.NutritionRepository, workoutRepo domain.WorkoutRepository, jwtSecret string, jwtExpirationHours int, portfolioRepos ...domain.UserPortfolioRepository) domain.UserUseCase {
	var portfolioRepo domain.UserPortfolioRepository
	if len(portfolioRepos) > 0 {
		portfolioRepo = portfolioRepos[0]
	}
	return &userUseCase{
		repo:               repo,
		portfolioRepo:      portfolioRepo,
		driRepo:            driRepo,
		nutriRepo:          nutriRepo,
		workoutRepo:        workoutRepo,
		jwtSecret:          jwtSecret,
		jwtExpirationHours: jwtExpirationHours,
	}
}

// Register creates a new user account. Hashes the password with bcrypt before storing.
func (u *userUseCase) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.AuthResponse, error) {
	if !isPasswordStrong(req.Password) {
		return nil, domain.ErrWeakPassword
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, domain.ErrInternalServerError
	}

	user := &domain.User{
		Username:          req.Username,
		Email:             strings.ToLower(req.Email),
		Password:          string(hashed),
		FullName:          req.FullName,
		HeightCm:          req.HeightCm,
		WeightKg:          req.WeightKg,
		DOB:               req.DOB,
		Gender:            req.Gender,
		ActivityLevel:     domain.ActivityLevel(req.ActivityLevel),
		DietaryPreference: req.DietaryPreference,
		MedicalConditions: req.MedicalConditions,
	}

	if err := u.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := u.generateTokens(ctx, user.ID, uuid.Nil)
	if err != nil {
		return nil, domain.ErrInternalServerError
	}

	return &domain.AuthResponse{Token: accessToken, RefreshToken: refreshToken, User: *user}, nil
}

// Login authenticates a user by email or username + password.
func (u *userUseCase) Login(ctx context.Context, req *domain.LoginRequest) (*domain.AuthResponse, error) {
	var user *domain.User
	var err error

	if strings.Contains(req.Identifier, "@") {
		user, err = u.repo.GetByEmail(ctx, strings.ToLower(req.Identifier))
	} else {
		user, err = u.repo.GetByUsername(ctx, req.Identifier)
	}

	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	accessToken, refreshToken, err := u.generateTokens(ctx, user.ID, uuid.Nil)
	if err != nil {
		return nil, domain.ErrInternalServerError
	}

	return &domain.AuthResponse{Token: accessToken, RefreshToken: refreshToken, User: *user}, nil
}

// RefreshTokens exchanges a valid refresh token for a new access token and refresh token.
func (u *userUseCase) RefreshTokens(ctx context.Context, req *domain.RefreshRequest) (*domain.AuthResponse, error) {
	hash := crypto.HashToken(req.RefreshToken)
	rt, err := u.repo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	// REUSE DETECTION (OWASP standard)
	if rt.Revoked {
		// Token was already used/revoked! Someone is replaying an old refresh token.
		// Kill the entire session family to protect the user.
		_ = u.repo.RevokeFamily(ctx, rt.FamilyID)
		return nil, domain.ErrReuseDetected
	}

	if time.Now().After(rt.ExpiresAt) {
		return nil, domain.ErrInvalidRefreshToken
	}

	// Generate new token pair under the SAME FamilyID
	accessToken, newRawRefreshToken, err := u.generateTokens(ctx, rt.UserID, rt.FamilyID)
	if err != nil {
		return nil, domain.ErrInternalServerError
	}
	newHash := crypto.HashToken(newRawRefreshToken)

	// Token rotation: Revoke the old token and mark it replaced by the new hash
	_ = u.repo.RevokeRefreshToken(ctx, hash, &newHash)

	// Fetch user details to populate AuthResponse
	user, err := u.repo.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	return &domain.AuthResponse{Token: accessToken, RefreshToken: newRawRefreshToken, User: *user}, nil
}

// UpdateProfile delegates profile updates to the repository, recalculating BMR/TDEE beforehand.
func (u *userUseCase) UpdateProfile(ctx context.Context, userID uuid.UUID, req *domain.UpdateProfileRequest) error {
	user, err := u.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	weight := user.WeightKg
	if req.WeightKg != nil {
		weight = *req.WeightKg
	}
	height := user.HeightCm
	if req.HeightCm != nil {
		height = *req.HeightCm
	}
	dob := user.DOB
	if req.DOB != nil {
		dob = req.DOB
	}
	gender := user.Gender
	if req.Gender != "" {
		gender = req.Gender
	}
	activity := user.ActivityLevel
	if req.ActivityLevel != "" {
		activity = domain.ActivityLevel(req.ActivityLevel)
	}

	if weight > 0 && height > 0 && dob != nil {
		calcService := service.NewHealthCalculationService()
		bmrVal := calcService.CalculateBMR(weight, height, *dob, gender)
		tdeeVal := calcService.CalculateTDEE(bmrVal, activity)

		bmrFloat := float64(bmrVal)
		tdeeFloat := float64(tdeeVal)
		req.BMR = &bmrFloat
		req.TDEE = &tdeeFloat
	}

	return u.repo.UpdateProfile(ctx, userID, req)
}

// GetProfile retrieves the full user profile by ID.
func (u *userUseCase) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return u.repo.GetByID(ctx, userID)
}

func (u *userUseCase) GetPortfolio(ctx context.Context, userID uuid.UUID) (*domain.UserPortfolioResponse, error) {
	user, err := u.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	portfolio := &domain.UserPortfolio{UserID: userID}
	if u.portfolioRepo != nil {
		loaded, err := u.portfolioRepo.GetPortfolio(ctx, userID)
		if err != nil {
			return nil, err
		}
		portfolio = loaded
	}
	return buildPortfolioResponse(user, portfolio), nil
}

func (u *userUseCase) UpdatePortfolio(ctx context.Context, userID uuid.UUID, req *domain.UserPortfolioRequest) (*domain.UserPortfolioResponse, error) {
	if u.portfolioRepo == nil {
		return nil, domain.ErrInternalServerError
	}
	user, err := u.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	portfolio := &domain.UserPortfolio{
		UserID:                userID,
		PreferredCuisines:     cleanStringSlice(req.PreferredCuisines),
		DislikedIngredients:   cleanStringSlice(req.DislikedIngredients),
		ExcludedIngredients:   cleanStringSlice(req.ExcludedIngredients),
		MealSchedule:          req.MealSchedule,
		DailyWaterTargetML:    req.DailyWaterTargetML,
		CalorieTargetOverride: req.CalorieTargetOverride,
		MacroSplitOverride:    req.MacroSplitOverride,
		Notes:                 strings.TrimSpace(req.Notes),
	}
	if portfolio.MealSchedule == nil {
		portfolio.MealSchedule = map[string]any{}
	}
	if portfolio.MacroSplitOverride == nil {
		portfolio.MacroSplitOverride = map[string]any{}
	}
	if err := u.portfolioRepo.UpsertPortfolio(ctx, portfolio); err != nil {
		return nil, err
	}
	loaded, err := u.portfolioRepo.GetPortfolio(ctx, userID)
	if err != nil {
		return nil, err
	}
	return buildPortfolioResponse(user, loaded), nil
}

// GetTargets calculates personalized daily nutritional targets from DRI data.
func (u *userUseCase) GetTargets(ctx context.Context, userID uuid.UUID) (*domain.UserTargetsResponse, error) {
	user, err := u.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// ── Step A: Compute age (graceful fallback to 25 if DOB unset) ────────
	age := 25
	if user.DOB != nil {
		age = computeAge(*user.DOB)
	}

	// ── Step B: Map to DRI demographic (fallback to adult male) ──────────
	gender := user.Gender
	if gender == "" {
		gender = "male"
	}
	lifeStage, ageRange := mapToDRIDemographic(gender, age)

	// ── Step C: Fetch DRI row ──────────────────────────────────────────────────
	dri, err := u.driRepo.GetByDemographic(ctx, lifeStage, ageRange)
	if err != nil {
		return nil, err
	}

	// ── Step D: Compute caloric target (TDEE or profile budget) ─────────────────
	caloricTarget := computeCalorieTarget(user)

	proteinRatio, fatRatio, carbRatio, hasMacroOverride := u.macroSplitRatios(ctx, userID)
	proteinKcal := caloricTarget * proteinRatio
	fatKcal := caloricTarget * fatRatio
	carbKcal := caloricTarget * carbRatio

	proteinG := math.Round(proteinKcal/4*10) / 10
	fatG := math.Round(fatKcal/9*10) / 10
	carbG := math.Round(carbKcal/4*10) / 10

	// Override protein with DRI EAR body-weight formula when available
	if !hasMacroOverride && dri.Ear.Macronutrients.ProteinGPerKg != nil && user.WeightKg > 0 {
		proteinG = math.Round(*dri.Ear.Macronutrients.ProteinGPerKg*user.WeightKg*10) / 10
	}

	// ── Step E: Build micronutrient targets from DRI RDA/AI ──────────────────
	micros := buildMicronutrients(dri.RdaAi)

	// ── Step F: Aggregate today's micronutrient intake from meal logs ─────────
	// Fetch today's logs (Food is eagerly loaded by the repository).
	today := time.Now()
	logs, logsErr := u.nutriRepo.GetDailyLogs(ctx, userID, today)
	if logsErr == nil && len(logs) > 0 {
		// Build a running sum: nutrientName → total consumed (numeric value only)
		sums := make(map[string]float64)
		for _, ml := range logs {
			ratio := ml.QuantityGrams / 100.0
			for rawName, rawVal := range ml.Food.Micronutrients {
				numeric := parseMicroVal(fmt.Sprintf("%v", rawVal))
				if numeric > 0 {
					sums[rawName] += numeric * ratio
				}
			}
		}
		// Inject sums into matching MicroTarget entries (case-insensitive prefix match)
		for i, m := range micros {
			for sumName, sumVal := range sums {
				if microNamesMatch(m.Name, sumName) {
					micros[i].Current = math.Round(sumVal*100) / 100
					break
				}
			}
		}
	}

	// ── Step G: Aggregate today's macro Current values ────────────────────────
	var totalCal, totalProt, totalFat, totalCarb float64
	for _, ml := range logs {
		totalCal += ml.CaloriesConsumed
		totalProt += ml.ProteinConsumed
		totalFat += ml.FatConsumed
		totalCarb += ml.CarbsConsumed
	}

	// ── Step H: Aggregate today's burned calories from workouts ───────────────
	burnedKcal, _ := u.workoutRepo.GetDailyBurnedCalories(ctx, userID, today)

	return &domain.UserTargetsResponse{
		TotalCalories: domain.TargetValue{Current: math.Round(totalCal*10) / 10, Target: caloricTarget},
		Burned:        burnedKcal,
		Macronutrients: map[string]domain.TargetValue{
			"protein":      {Current: math.Round(totalProt*10) / 10, Target: proteinG},
			"fat":          {Current: math.Round(totalFat*10) / 10, Target: fatG},
			"carbohydrate": {Current: math.Round(totalCarb*10) / 10, Target: carbG},
		},
		Micronutrients: micros,
	}, nil
}

// generateTokens produces a signed JWT (15 mins) and a secure random refresh token (7 days).
func (u *userUseCase) generateTokens(ctx context.Context, userID uuid.UUID, familyID uuid.UUID) (string, string, error) {
	// Access Token: strict 15-minute lifespan in production (override via config for specific needs)
	// Currently jwtExpirationHours represents hours, let's treat it as minutes for access if we enforce 15m
	// Or we just hardcode 15m as per the security design plan.
	accessExpiry := 15 * time.Minute
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(accessExpiry).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(u.jwtSecret))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	// Refresh Token: 7 days lifespan
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	rawRefreshToken := base64.URLEncoding.EncodeToString(b)
	hashedToken := crypto.HashToken(rawRefreshToken)

	fID := familyID
	if fID == uuid.Nil {
		fID = uuid.New()
	}

	rtRecord := &domain.RefreshToken{
		UserID:    userID,
		TokenHash: hashedToken,
		FamilyID:  fID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	if err := u.repo.SaveRefreshToken(ctx, rtRecord); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	return signed, rawRefreshToken, nil
}

// ─── Private Helpers ──────────────────────────────────────────────────────────

// isPasswordStrong verifies that a password is at least 8 chars long, contains
// an uppercase letter, a lowercase letter, and a number.
func isPasswordStrong(pw string) bool {
	if len(pw) < 8 {
		return false
	}
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(pw)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(pw)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(pw)
	return hasUpper && hasLower && hasNumber
}

func computeAge(dob time.Time) int {
	now := time.Now()
	years := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		years--
	}
	return years
}

// mapToDRIDemographic maps a user's gender and age to the DRI life stage categories
// stored in our database. Ages use en-dash (–) as per the original DRIs.json.
func mapToDRIDemographic(gender string, age int) (lifeStage, ageRange string) {
	g := strings.ToLower(gender)
	switch {
	case g == "male" || g == "males":
		lifeStage = "Males"
	case g == "female" || g == "females":
		lifeStage = "Females"
	default:
		lifeStage = "Males" // safe default
	}

	switch {
	case age < 1:
		// Infant — use a general adult fallback; infants need specific paediatric logic
		lifeStage = "Infants"
		ageRange = "0\u20136 mo"
	case age < 4:
		lifeStage = "Children"
		ageRange = "1\u20133 y"
	case age <= 30:
		ageRange = "19\u201330 y"
	default:
		// No 31–50 y row in our seeded data; re-use 19–30 as a reasonable adult proxy
		ageRange = "19\u201330 y"
	}

	return lifeStage, ageRange
}

// computeCalorieTarget picks the best daily calorie figure from the user's stored values.
func computeCalorieTarget(user *domain.User) float64 {
	if user.WeeklyCalorieBudget > 0 {
		return math.Round(user.WeeklyCalorieBudget / 7)
	}
	if user.TDEE > 0 {
		return math.Round(user.TDEE)
	}
	// Harris-Benedict BMR as a last resort
	if user.WeightKg > 0 && user.HeightCm > 0 && user.DOB != nil {
		age := computeAge(*user.DOB)
		var bmr float64
		if strings.ToLower(user.Gender) == "female" {
			bmr = 655.1 + (9.563 * user.WeightKg) + (1.850 * user.HeightCm) - (4.676 * float64(age))
		} else {
			bmr = 66.47 + (13.75 * user.WeightKg) + (5.003 * user.HeightCm) - (6.755 * float64(age))
		}
		return math.Round(bmr * 1.375) // lightly active multiplier
	}
	return 2000 // absolute last-resort default
}

func (u *userUseCase) macroSplitRatios(ctx context.Context, userID uuid.UUID) (protein, fat, carb float64, overridden bool) {
	protein, fat, carb = 0.25, 0.30, 0.45
	if u.portfolioRepo == nil {
		return protein, fat, carb, false
	}
	portfolio, err := u.portfolioRepo.GetPortfolio(ctx, userID)
	if err != nil || portfolio == nil || len(portfolio.MacroSplitOverride) == 0 {
		return protein, fat, carb, false
	}

	p := macroSplitValue(portfolio.MacroSplitOverride, "protein")
	f := macroSplitValue(portfolio.MacroSplitOverride, "fat")
	c := macroSplitValue(portfolio.MacroSplitOverride, "carbohydrate")
	if c == 0 {
		c = macroSplitValue(portfolio.MacroSplitOverride, "carbs")
	}
	total := p + f + c
	if total <= 0 {
		return protein, fat, carb, false
	}
	if total > 1.5 {
		p, f, c = p/100, f/100, c/100
		total = p + f + c
	}
	if total < 0.95 || total > 1.05 {
		return protein, fat, carb, false
	}
	return p, f, c, true
}

func macroSplitValue(values map[string]any, key string) float64 {
	raw, ok := values[key]
	if !ok || raw == nil {
		return 0
	}
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func buildPortfolioResponse(user *domain.User, portfolio *domain.UserPortfolio) *domain.UserPortfolioResponse {
	if portfolio == nil {
		portfolio = &domain.UserPortfolio{UserID: user.ID}
	}
	mealSchedule := map[string]any(portfolio.MealSchedule)
	if mealSchedule == nil {
		mealSchedule = map[string]any{}
	}
	macroSplit := map[string]any(portfolio.MacroSplitOverride)
	if macroSplit == nil {
		macroSplit = map[string]any{}
	}
	return &domain.UserPortfolioResponse{
		UserID:                user.ID,
		HeightCm:              user.HeightCm,
		WeightKg:              user.WeightKg,
		DOB:                   user.DOB,
		Gender:                user.Gender,
		ActivityLevel:         user.ActivityLevel,
		BMR:                   user.BMR,
		TDEE:                  user.TDEE,
		GoalType:              user.GoalType,
		DietaryPreference:     user.DietaryPreference,
		Allergies:             user.Allergies,
		MedicalConditions:     user.MedicalConditions,
		PreferredCuisines:     []string(portfolio.PreferredCuisines),
		DislikedIngredients:   []string(portfolio.DislikedIngredients),
		ExcludedIngredients:   []string(portfolio.ExcludedIngredients),
		MealSchedule:          mealSchedule,
		DailyWaterTargetML:    portfolio.DailyWaterTargetML,
		CalorieTargetOverride: portfolio.CalorieTargetOverride,
		MacroSplitOverride:    macroSplit,
		Notes:                 portfolio.Notes,
		UpdatedAt:             portfolio.UpdatedAt,
	}
}

func cleanStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		cleaned := strings.ToLower(strings.TrimSpace(value))
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

// derefF64 safely dereferences a *float64 and returns 0 if nil.
func derefF64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// buildMicronutrients maps non-zero DRI RDA/AI element and vitamin values to MicroTarget slice.
func buildMicronutrients(req domain.DRIRequirements) []domain.MicroTarget {
	micros := []domain.MicroTarget{}

	add := func(name string, unit string, val *float64) {
		if val != nil && *val > 0 {
			micros = append(micros, domain.MicroTarget{
				Name:    name,
				Unit:    unit,
				Target:  derefF64(val),
				Current: 0,
			})
		}
	}

	e := req.Elements
	add("Calcium", "mg", e.CalciumMg)
	add("Chromium", "mcg", e.ChromiumMcg)
	add("Copper", "mcg", e.CopperMcg)
	add("Fluoride", "mg", e.FluorideMg)
	add("Iodine", "mcg", e.IodineMcg)
	add("Iron", "mg", e.IronMg)
	add("Magnesium", "mg", e.MagnesiumMg)
	add("Manganese", "mg", e.ManganeseMg)
	add("Molybdenum", "mcg", e.MolybdenumMcg)
	add("Phosphorus", "mg", e.PhosphorusMg)
	add("Selenium", "mcg", e.SeleniumMcg)
	add("Zinc", "mg", e.ZincMg)
	add("Potassium", "mg", e.PotassiumMg)
	add("Sodium", "mg", e.SodiumMg)
	add("Chloride", "g", e.ChlorideG)

	v := req.Vitamins
	add("Vitamin A", "mcg", v.VitaminAMcg)
	add("Vitamin C", "mg", v.VitaminCMg)
	add("Vitamin D", "mcg", v.VitaminDMcg)
	add("Vitamin E", "mg", v.VitaminEMg)
	add("Vitamin K", "mcg", v.VitaminKMcg)
	add("Thiamin (B1)", "mg", v.ThiaminMg)
	add("Riboflavin (B2)", "mg", v.RiboflavinMg)
	add("Niacin (B3)", "mg", v.NiacinMg)
	add("Vitamin B6", "mg", v.VitaminB6Mg)
	add("Folate (B9)", "mcg", v.FolateMcg)
	add("Vitamin B12", "mcg", v.VitaminB12Mcg)
	add("Pantothenic Acid (B5)", "mg", v.PantothenicAcidMg)
	add("Biotin (B7)", "mcg", v.BiotinMcg)
	add("Choline", "mg", v.CholineMg)

	return micros
}

// reNumeric matches the first numeric token (incl. decimals) in a string like "10.800000 mg".
var reNumeric = regexp.MustCompile(`[\d]+\.?[\d]*`)

// parseMicroVal extracts the leading numeric value from a "value unit" string.
// Example: "10.800000 mg" → 10.8, "434.6 µg" → 434.6, "0.5" → 0.5.
func parseMicroVal(s string) float64 {
	match := reNumeric.FindString(strings.TrimSpace(s))
	if match == "" {
		return 0
	}
	v, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return v
}

// microNamesMatch returns true when the DRI display name (e.g. "Vitamin B12") and
// the raw DB key (e.g. "Vitamin B12" or "vitamin b12") refer to the same nutrient.
// Uses case-insensitive comparison on the canonical name vs the DB key prefix.
func microNamesMatch(driName, dbKey string) bool {
	d := strings.ToLower(strings.TrimSpace(driName))
	k := strings.ToLower(strings.TrimSpace(dbKey))
	// Direct match or one is a prefix of the other
	return d == k || strings.HasPrefix(k, d) || strings.HasPrefix(d, k)
}
