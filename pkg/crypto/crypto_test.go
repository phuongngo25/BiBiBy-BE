package crypto

import (
	"testing"
)

func TestAESEncryptionDecryption(t *testing.T) {
	keyv1 := "32_byte_secret_key_v1_0000000000"
	keyv2 := "32_byte_secret_key_v2_0000000000"
	keyring := map[string]string{
		"v1": keyv1,
		"v2": keyv2,
	}

	userID := "user-123"
	aad := BuildAAD(userID)
	plaintext := "High blood pressure"

	// 1. Encrypt with v1
	ciphertext, err := Encrypt(plaintext, keyv1, aad, "v1")
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// 2. Decrypt with correct AAD
	decrypted, rot, err := Decrypt(ciphertext, keyring, "v1", aad)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypted text mismatch. Got %s, want %s", decrypted, plaintext)
	}
	if rot {
		t.Error("Expected rot to be false when versions match")
	}

	// 3. Decrypt with WRONG AAD (Should fail)
	wrongAAD := BuildAAD("hacker-999")
	_, _, err = Decrypt(ciphertext, keyring, "v1", wrongAAD)
	if err == nil {
		t.Error("Expected decryption to fail with incorrect AAD")
	}

	// 4. Test Rotation Detection (Active is v2, token is v1)
	_, rot, err = Decrypt(ciphertext, keyring, "v2", aad)
	if err != nil {
		t.Fatalf("Decryption failed during rotation test: %v", err)
	}
	if !rot {
		t.Error("Expected rot to be true when version mismatch")
	}
}

func TestBlindIndexNormalization(t *testing.T) {
	key := "32_byte_hmac_key_000000000000000"

	// Matches diacritics
	idx1 := BlindIndex("Tiểu Đường", key)
	idx2 := BlindIndex("tieu duong", key)

	if idx1 == "" || idx2 == "" {
		t.Fatal("Blind index should not be empty")
	}

	if idx1 != idx2 {
		t.Errorf("Blind index mismatch for Vietnamese normalization. Got %s and %s", idx1, idx2)
	}

	// Versioning check
	if !testing.Short() {
		if idx1[:8] != "bidx:v1:" {
			t.Errorf("Expected bidx:v1: prefix, got %s", idx1[:8])
		}
	}
}

func TestEmptyInputHandling(t *testing.T) {
	key := "32_byte_secret_key_v1_0000000000"
	hmacKey := "32_byte_hmac_key_000000000000000"
	aad := BuildAAD("user-1")

	if enc, _ := Encrypt("", key, aad, "v1"); enc != "" {
		t.Error("Empty encryption should return empty string")
	}

	if idx := BlindIndex("", hmacKey); idx != "" {
		t.Error("Empty blind index should return empty string")
	}
}
