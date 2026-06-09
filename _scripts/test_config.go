package main

import (
	"log"
	"os"
	"nutrix-backend/config"
)

func main() {
	// Giả lập quên config GRPC_AI_HOST
	os.Setenv("ENCRYPTION_KEYS", `{"v1":"0123456789abcdef0123456789abcdef"}`)
	os.Setenv("ACTIVE_KEY_VERSION", "v1")
	os.Setenv("HMAC_KEY", "0123456789abcdef0123456789abcdef")
	os.Setenv("GRPC_AI_HOST", "") 
	
	log.Println("Testing Config Load without GRPC_AI_HOST...")
	cfg := config.LoadConfig()
	log.Println("Config loaded successfully:", cfg.GRPCAIHost)
}
