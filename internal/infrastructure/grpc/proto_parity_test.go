package grpc_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hashFile computes the SHA-256 hash of a file.
func hashFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file %s: %v", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("Failed to hash file %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashNormalizedProtoFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])
}

// TestProtoParityGate ensures that the protobuf definitions in nutrix-contracts
// are exactly the same as the single source of truth in AI_server/api/proto.
func TestProtoParityGate(t *testing.T) {
	// Paths relative to the go_backend root.
	// Since tests run in the package dir (internal/infrastructure/grpc), we go up 4 levels.
	aiServerProtoDir := filepath.Join("..", "..", "..", "..", "AI_server", "api", "proto", "v1")
	contractsProtoDir := filepath.Join("..", "..", "..", "..", "nutrix-contracts", "proto")

	// Ensure the directories exist
	if _, err := os.Stat(aiServerProtoDir); os.IsNotExist(err) {
		t.Skipf("AI_server proto dir %s does not exist, skipping parity check. (Expected in local dev or CI)", aiServerProtoDir)
	}

	protosToCheck := []string{"nutrition_intelligence.proto", "common.proto"}

	for _, protoFile := range protosToCheck {
		aiPath := filepath.Join(aiServerProtoDir, protoFile)
		contractPath := filepath.Join(contractsProtoDir, protoFile)

		aiHash := hashNormalizedProtoFile(t, aiPath)
		contractHash := hashNormalizedProtoFile(t, contractPath)

		if aiHash != contractHash {
			t.Errorf("CRITICAL Proto Drift Detected! %s differs between AI_server and nutrix-contracts.\nAI Server Hash: %s\nContracts Hash: %s\nPlease sync them to avoid Gateway Architecture Violation.", protoFile, aiHash, contractHash)
		}
	}
}

func TestInferenceProtoParityGate(t *testing.T) {
	workspaceRoot := filepath.Join("..", "..", "..", "..")
	aiPath := filepath.Join(workspaceRoot, "AI_server", "Computer_Vision", "proto", "inference.proto")
	goPath := filepath.Join(workspaceRoot, "go_backend", "api", "proto", "inference.proto")
	flutterPath := filepath.Join(workspaceRoot, "nutrix", "proto", "inference.proto")

	if _, err := os.Stat(aiPath); os.IsNotExist(err) {
		t.Skipf("AI_server CV proto %s does not exist, skipping parity check", aiPath)
	}

	aiHash := hashNormalizedProtoFile(t, aiPath)
	for _, path := range []string{goPath, flutterPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Expected inference proto %s: %v", path, err)
		}
		if got := hashNormalizedProtoFile(t, path); got != aiHash {
			t.Errorf("CRITICAL Inference Proto Drift Detected! %s differs from AI_server Computer_Vision proto.\nAI Server Hash: %s\nOther Hash: %s", path, aiHash, got)
		}
	}
}
