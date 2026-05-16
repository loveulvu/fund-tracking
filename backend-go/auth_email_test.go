package main

import (
	"testing"
	"time"
)

func TestGenerateVerificationCodeFormat(t *testing.T) {
	code, err := generateVerificationCode()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !isValidVerificationCodeFormat(code) {
		t.Fatalf("expected six digit code, got %q", code)
	}
}

func TestHashVerificationCodeMatchesSameCode(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	first, err := hashVerificationCode("123456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	second, err := hashVerificationCode("123456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if first == "" {
		t.Fatal("expected non-empty hash")
	}
	if first != second {
		t.Fatal("expected same code to produce same hash")
	}
}

func TestHashVerificationCodeRejectsWrongCode(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	expected, err := hashVerificationCode("123456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	actual, err := hashVerificationCode("654321")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if expected == actual {
		t.Fatal("expected different codes to produce different hashes")
	}
}

func TestHashVerificationCodeRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := hashVerificationCode("123456"); err == nil {
		t.Fatal("expected missing JWT_SECRET to return error")
	}
}

func TestIsValidVerificationCodeFormat(t *testing.T) {
	validCodes := []string{"000000", "123456", "999999"}
	for _, code := range validCodes {
		if !isValidVerificationCodeFormat(code) {
			t.Fatalf("expected %q to be valid", code)
		}
	}

	invalidCodes := []string{"", "12345", "1234567", "12345a", "12 456"}
	for _, code := range invalidCodes {
		if isValidVerificationCodeFormat(code) {
			t.Fatalf("expected %q to be invalid", code)
		}
	}
}

func TestCanAttemptEmailVerification(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	user := User{
		EmailVerificationCodeHash:  "hash",
		EmailVerificationExpiresAt: now.Add(10 * time.Minute),
		EmailVerificationAttempts:  4,
	}
	if !canAttemptEmailVerification(user, now) {
		t.Fatal("expected verification attempt to be allowed")
	}

	user.EmailVerificationExpiresAt = now.Add(-time.Second)
	if canAttemptEmailVerification(user, now) {
		t.Fatal("expected expired code to be rejected")
	}

	user.EmailVerificationExpiresAt = now.Add(10 * time.Minute)
	user.EmailVerificationAttempts = 5
	if canAttemptEmailVerification(user, now) {
		t.Fatal("expected attempts >= 5 to be rejected")
	}

	user.EmailVerificationAttempts = 0
	user.EmailVerificationCodeHash = ""
	if canAttemptEmailVerification(user, now) {
		t.Fatal("expected empty hash to be rejected")
	}
}

func TestIsEmailVerifiedForLoginCompatibility(t *testing.T) {
	if !isEmailVerifiedForLogin(User{}) {
		t.Fatal("expected missing emailVerified to allow legacy login")
	}

	verified := true
	if !isEmailVerifiedForLogin(User{EmailVerified: &verified}) {
		t.Fatal("expected verified user to log in")
	}

	unverified := false
	if isEmailVerifiedForLogin(User{EmailVerified: &unverified}) {
		t.Fatal("expected explicitly unverified user to be blocked")
	}
}

func TestEmailVerificationCooldownRemaining(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	retryAfter, coolingDown := emailVerificationCooldownRemaining(now.Add(-30*time.Second), now)
	if !coolingDown {
		t.Fatal("expected resend cooldown to be active")
	}
	if retryAfter <= 0 || retryAfter > 60 {
		t.Fatalf("expected retryAfter in range, got %d", retryAfter)
	}

	if _, coolingDown := emailVerificationCooldownRemaining(now.Add(-61*time.Second), now); coolingDown {
		t.Fatal("expected cooldown to be over")
	}
}
