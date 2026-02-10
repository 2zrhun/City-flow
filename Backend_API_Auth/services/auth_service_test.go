package services

import (
	"testing"

	"traffic-prediction-api/config"
)

func newTestAuthService() *AuthService {
	return NewAuthService(config.JWTConfig{
		Secret:      "test-secret-key",
		ExpiryHours: 24,
	})
}

func TestHashAndCheckPassword(t *testing.T) {
	svc := newTestAuthService()

	hash, err := svc.HashPassword("mypassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	if hash == "mypassword123" {
		t.Fatal("hash should not equal plaintext")
	}

	if !svc.CheckPassword(hash, "mypassword123") {
		t.Error("CheckPassword should return true for correct password")
	}
	if svc.CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	svc := newTestAuthService()

	token, err := svc.GenerateToken(1, "user@test.com", "user")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.Email != "user@test.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "user@test.com")
	}
	if claims.Role != "user" {
		t.Errorf("Role = %q, want %q", claims.Role, "user")
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	svc := newTestAuthService()

	_, err := svc.ValidateToken("invalid.token.string")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	svc1 := NewAuthService(config.JWTConfig{Secret: "secret-1", ExpiryHours: 24})
	svc2 := NewAuthService(config.JWTConfig{Secret: "secret-2", ExpiryHours: 24})

	token, _ := svc1.GenerateToken(1, "user@test.com", "user")

	_, err := svc2.ValidateToken(token)
	if err == nil {
		t.Error("expected error when validating with wrong secret")
	}
}

func TestTokenContainsClaims(t *testing.T) {
	svc := newTestAuthService()

	token, _ := svc.GenerateToken(42, "admin@city.flow", "admin")
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("UserID = %d, want 42", claims.UserID)
	}
	if claims.Email != "admin@city.flow" {
		t.Errorf("Email = %q", claims.Email)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q", claims.Role)
	}
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	}
	if claims.IssuedAt == nil {
		t.Error("IssuedAt should be set")
	}
}

func TestHashPasswordDifferentEachTime(t *testing.T) {
	svc := newTestAuthService()

	hash1, _ := svc.HashPassword("same-password")
	hash2, _ := svc.HashPassword("same-password")

	if hash1 == hash2 {
		t.Error("bcrypt hashes should differ due to random salt")
	}

	// But both should validate
	if !svc.CheckPassword(hash1, "same-password") {
		t.Error("hash1 should validate")
	}
	if !svc.CheckPassword(hash2, "same-password") {
		t.Error("hash2 should validate")
	}
}
