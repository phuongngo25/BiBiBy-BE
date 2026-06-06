package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/infrastructure"
)

func main() {
	client, err := infrastructure.NewGrpcAIClient("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create grpc client: %v", err)
	}
	defer func() {
		if closer, ok := client.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	// Đọc file ảnh
	phoBytes, err := ioutil.ReadFile("uploads/pho.png")
	if err != nil {
		log.Fatalf("Failed to read pho.png: %v", err)
	}



	fmt.Println("==================================================")
	fmt.Println("🚀 SPRINT 11.6: PRODUCTION TRUTH AUDIT")
	fmt.Println("==================================================")

	runAudit("Audit E", "Pho (Production LLM Enrichment)", phoBytes, []string{}, client)
	runAudit("Audit F", "Gout + Pho (Testing Recommendation)", phoBytes, []string{"D_GOUT"}, client)
	
	fmt.Println("\n--- Audit G (Idempotency) ---")
	fmt.Println("Gửi 3 request AnalyzeMealImage liên tiếp cho Pho...")
	for i := 0; i < 3; i++ {
		_, _ = client.AnalyzeMealImage(context.Background(), phoBytes, []string{})
	}
	fmt.Println("✅ Đã gửi 3 requests. Vui lòng check Cypher query thủ công để chứng minh Graph không bị duplicate nodes!")
}

func runAudit(name, foodDesc string, img []byte, diseases []string, client domain.InferencePort) {
	fmt.Printf("\n--- %s ---\n", name)
	fmt.Printf("Food: %s\n", foodDesc)
	fmt.Printf("User Diseases: %v\n", diseases)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.AnalyzeMealImage(ctx, img, diseases)
	if err != nil {
		fmt.Printf("Result: ERROR - %v\n", err)
		return
	}

	// In ra JSON đẹp
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("Result:\n%s\n", string(b))
}
