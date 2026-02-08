package utilities

import (
	"financial-Assistant/internal/mainservice/models"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGenerateAndDecodeAccessToken(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")

	user := models.User{
		ID:         primitive.NewObjectID(),
		Name:       "Test",
		LastName:   "User",
		Email:      "test@example.com",
		TypeClient: "Quartz",
	}
	secretKey := "test-access-secret-key"

	token, _, err := GenerateToken(user, "device-123", secretKey)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := DecodeToken(token, secretKey, "access")
	if err != nil {
		t.Fatalf("DecodeToken failed: %v", err)
	}

	if claims["email"] != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, claims["email"])
	}
	if claims["type"] != "access" {
		t.Errorf("expected type 'access', got %q", claims["type"])
	}
	if claims["deviceid"] != "device-123" {
		t.Errorf("expected deviceid 'device-123', got %q", claims["deviceid"])
	}
}

func TestGenerateAndDecodeRefreshToken(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")

	user := models.User{
		ID:         primitive.NewObjectID(),
		Name:       "Test",
		Email:      "test@example.com",
		TypeClient: "Quartz",
	}
	secretKey := "test-refresh-secret-key"

	token, _, err := GenerateRefreshToken(user, "device-456", secretKey)
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	claims, err := DecodeToken(token, secretKey, "refresh")
	if err != nil {
		t.Fatalf("DecodeToken failed: %v", err)
	}

	if claims["type"] != "refresh" {
		t.Errorf("expected type 'refresh', got %q", claims["type"])
	}
	if claims["deviceid"] != "device-456" {
		t.Errorf("expected deviceid 'device-456', got %q", claims["deviceid"])
	}
}

func TestDecodeTokenRejectsWrongType(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")

	user := models.User{
		ID:    primitive.NewObjectID(),
		Email: "test@example.com",
	}
	secretKey := "test-key"

	// Generate access token, try to decode as refresh
	accessToken, _, err := GenerateToken(user, "device-1", secretKey)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = DecodeToken(accessToken, secretKey, "refresh")
	if err == nil {
		t.Error("expected error when decoding access token as refresh, got nil")
	}
}

func TestDecodeTokenRejectsWrongSecret(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")

	user := models.User{
		ID:    primitive.NewObjectID(),
		Email: "test@example.com",
	}

	token, _, err := GenerateToken(user, "device-1", "correct-secret")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = DecodeToken(token, "wrong-secret", "access")
	if err == nil {
		t.Error("expected error when decoding with wrong secret, got nil")
	}
}

func TestHashToken(t *testing.T) {
	token := "some-jwt-token-value"
	hash1 := HashToken(token)
	hash2 := HashToken(token)

	if hash1 != hash2 {
		t.Error("same token should produce same hash")
	}

	hash3 := HashToken("different-token")
	if hash1 == hash3 {
		t.Error("different tokens should produce different hashes")
	}

	if len(hash1) != 64 {
		t.Errorf("SHA-256 hex hash should be 64 chars, got %d", len(hash1))
	}
}
