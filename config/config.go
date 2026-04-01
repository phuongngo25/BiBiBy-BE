package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application-level configuration loaded from the environment.
type Config struct {
	Port               string
	DBDSN              string
	JWTSecret          string
	JWTExpirationHours int
	SpoonacularAPIKey  string
	RapidAPIKey        string
	GoogleClientID     string
}

// LoadConfig reads from .env (if present) then from the process environment.
// It logs warnings for missing security-critical variables but never panics,
// so the server can still boot in basic "dev" mode.
func LoadConfig() *Config {
	// Load .env file if present — silently skip if missing (production uses real env vars)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — using system environment variables.")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "insecure-dev-secret-change-me"
		log.Println("WARNING: JWT_SECRET not set. Using insecure default. Please set this in your .env file.")
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		log.Println("WARNING: GOOGLE_CLIENT_ID not set. Google Sign-In verification may fail.")
	}

	jwtHours := 72
	if h := os.Getenv("JWT_EXPIRATION_HOURS"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil {
			jwtHours = parsed
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Port:               port,
		DBDSN:              os.Getenv("DB_DSN"),
		JWTSecret:          jwtSecret,
		JWTExpirationHours: jwtHours,
		SpoonacularAPIKey:  os.Getenv("SPOONACULAR_API_KEY"),
		RapidAPIKey:        os.Getenv("RAPIDAPI_KEY"),
		GoogleClientID:     googleClientID,
	}
}
