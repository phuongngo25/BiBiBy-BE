package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"

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
	// AllowedOrigins is a comma-separated list of origins permitted by CORS.
	AllowedOrigins     []string
	// Data Protection Phase 2
	EncryptionKeys     map[string]string
	ActiveKeyVersion   string
	HMACKey            string
	// Distributed Systems
	RedisURL           string
	RedisPassword      string
	RabbitMQURL        string
	// Internal Services
	GRPCAIHost         string
	GRPCAIPort         string
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

	// CORS origins — comma-separated; falls back to localhost dev origins
	allowedOriginsRaw := os.Getenv("ALLOWED_ORIGINS")
	var allowedOrigins []string
	if allowedOriginsRaw != "" {
		for _, o := range strings.Split(allowedOriginsRaw, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}
	if len(allowedOrigins) == 0 {
		// Safe dev defaults — localhost on common ports
		allowedOrigins = []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://localhost:5173", // Vite dev server
		}
		log.Println("WARNING: ALLOWED_ORIGINS not set. Defaulting to localhost origins.")
	}

	// ─── Data Protection Phase 2 (SecOps Hardening) ──────────────────────────
	encryptionKeysRaw := os.Getenv("ENCRYPTION_KEYS")
	if encryptionKeysRaw == "" {
		panic("CRITICAL: ENCRYPTION_KEYS environment variable is missing. Terminating for safety.")
	}

	var encryptionKeys map[string]string
	if err := json.Unmarshal([]byte(encryptionKeysRaw), &encryptionKeys); err != nil {
		panic("CRITICAL: ENCRYPTION_KEYS is not a valid JSON map. Terminating for safety.")
	}

	activeKeyVersion := os.Getenv("ACTIVE_KEY_VERSION")
	if activeKeyVersion == "" {
		panic("CRITICAL: ACTIVE_KEY_VERSION is missing. Terminating for safety.")
	}

	activeKey, ok := encryptionKeys[activeKeyVersion]
	if !ok {
		panic("CRITICAL: ACTIVE_KEY_VERSION '" + activeKeyVersion + "' not found in ENCRYPTION_KEYS. Terminating for safety.")
	}

	if len(activeKey) != 32 {
		panic("CRITICAL: Active encryption key must be exactly 32 bytes (AES-256). Terminating for safety.")
	}

	hmacKey := os.Getenv("HMAC_KEY")
	if hmacKey == "" {
		panic("CRITICAL: HMAC_KEY is missing. Terminating for safety.")
	}
	if len(hmacKey) != 32 {
		panic("CRITICAL: HMAC_KEY must be exactly 32 bytes. Terminating for safety.")
	}

	grpcAIHost := os.Getenv("GRPC_AI_HOST")
	if grpcAIHost == "" {
		panic("CRITICAL: GRPC_AI_HOST environment variable is missing. Terminating for safety.")
	}
	
	grpcAIPort := os.Getenv("GRPC_AI_PORT")
	if grpcAIPort == "" {
		panic("CRITICAL: GRPC_AI_PORT environment variable is missing. Terminating for safety.")
	}

	return &Config{
		Port:               port,
		DBDSN:              os.Getenv("DB_DSN"),
		JWTSecret:          jwtSecret,
		JWTExpirationHours: jwtHours,
		SpoonacularAPIKey:  os.Getenv("SPOONACULAR_API_KEY"),
		RapidAPIKey:        os.Getenv("RAPIDAPI_KEY"),
		GoogleClientID:     googleClientID,
		AllowedOrigins:     allowedOrigins,
		EncryptionKeys:     encryptionKeys,
		ActiveKeyVersion:   activeKeyVersion,
		HMACKey:            hmacKey,
		RedisURL:           os.Getenv("REDIS_URL"),
		RedisPassword:      os.Getenv("REDIS_PASSWORD"),
		RabbitMQURL:        os.Getenv("RABBITMQ_URL"),
		GRPCAIHost:         grpcAIHost,
		GRPCAIPort:         grpcAIPort,
	}
}
