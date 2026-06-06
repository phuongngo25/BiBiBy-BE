package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func main() {
	// Generate a dummy token
	secret := "a,5W|/&?DThQv(r?m:4423nB}BuiXX@ZVhFa9fKd6(ts8{v<ppP#ZtY7^%L_n410"
	userID := uuid.New().String()
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	
	tokenString, _ := token.SignedString([]byte(secret))
	
	// Prepare requests
	client := &http.Client{}
	today := time.Now().Format("2006-01-02")
	endpoints := []string{
		"http://localhost:8080/api/v1/nutrition/analytics/day?date=" + today,
		"http://localhost:8080/api/v1/nutrition/analytics/weekly",
		"http://localhost:8080/api/v1/nutrition/analytics/monthly?month=" + today[:7],
	}
	
	for _, ep := range endpoints {
		req, _ := http.NewRequest("GET", ep, nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error requesting %s: %v\n", ep, err)
			continue
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		
		fmt.Printf("Endpoint: %s\n", ep)
		fmt.Printf("Status: %d\n", resp.StatusCode)
		
		var prettyJSON map[string]interface{}
		if err := json.Unmarshal(body, &prettyJSON); err == nil {
			pretty, _ := json.MarshalIndent(prettyJSON, "", "  ")
			fmt.Printf("Response:\n%s\n\n", string(pretty))
		} else {
			fmt.Printf("Raw Response:\n%s\n\n", string(body))
		}
	}
}
