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

	comTamBytes, err := ioutil.ReadFile("uploads/com_tam.png")
	if err != nil {
		comTamBytes = phoBytes // fallback
	}

	fmt.Println("==================================================")
	fmt.Println("🚀 SPRINT 11.5: RUNTIME TRUTH AUDIT")
	fmt.Println("==================================================")

	runAudit("Audit A", "Tôm hấp (dùng tạm com_tam.png)", comTamBytes, []string{"D_SEAFOOD_ALLERGY"}, client)
	runAudit("Audit B", "Phở bò (pho.png)", phoBytes, []string{"D_GOUT"}, client)
	runAudit("Audit C", "Salad (Healthy - pho.png)", phoBytes, []string{}, client)
	
	// Audit D là trường hợp Neo4j OFF. 
	// Nếu bạn đang bật Neo4j, phần này sẽ trả về như Audit bình thường.
	// Để test chính xác Audit D, bạn cần stop Neo4j container trước.
	runAudit("Audit D", "Neo4j OFF test", phoBytes, []string{}, client)
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
