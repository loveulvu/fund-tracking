package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const authClaimsKey contextKey = "authClaims"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
	ID      string `json:"id"`
	Email   string `json:"email"`
}
type User struct {
	ID                         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email                      string             `bson:"email" json:"email"`
	PasswordHash               string             `bson:"password_hash" json:"-"`
	CreatedAt                  time.Time          `bson:"created_at" json:"created_at"`
	EmailVerified              *bool              `bson:"emailVerified,omitempty" json:"emailVerified"`
	EmailVerificationCodeHash  string             `bson:"emailVerificationCodeHash,omitempty" json:"-"`
	EmailVerificationExpiresAt time.Time          `bson:"emailVerificationExpiresAt,omitempty" json:"-"`
	EmailVerificationAttempts  int                `bson:"emailVerificationAttempts,omitempty" json:"-"`
	EmailVerificationSentAt    time.Time          `bson:"emailVerificationSentAt,omitempty" json:"-"`
	EmailVerifiedAt            time.Time          `bson:"emailVerifiedAt,omitempty" json:"emailVerifiedAt,omitempty"`
}
type AuthClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}
type RegisterResponse struct {
	Message              string `json:"message"`
	ID                   string `json:"id"`
	Email                string `json:"email"`
	RequiresVerification bool   `json:"requiresVerification"`
}

type VerifyEmailCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type ResendEmailCodeRequest struct {
	Email string `json:"email"`
}

type VerifyEmailCodeResponse struct {
	Status        string `json:"status"`
	Message       string `json:"message"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}

type ResendEmailCodeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type AuthErrorResponse struct {
	Error                string `json:"error"`
	RequiresVerification bool   `json:"requiresVerification,omitempty"`
	Email                string `json:"email,omitempty"`
	RetryAfterSeconds    int    `json:"retryAfterSeconds,omitempty"`
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := normalizeEmail(req.Email)
	password := req.Password
	if !isValidEmailFormat(email) {
		writeAuthJSONError(w, http.StatusBadRequest, "Email is required", AuthErrorResponse{})
		return
	}
	if len(password) < 6 {
		writeAuthJSONError(w, http.StatusBadRequest, "Password must be at least 6 characters", AuthErrorResponse{})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := mongoClient.Database("fund_tracking").Collection("users")

	var existing User
	err = collection.FindOne(ctx, bson.M{"email": email}).Decode(&existing)

	if err == nil {
		writeAuthJSONError(w, http.StatusConflict, "Email already registered", AuthErrorResponse{})
		return
	}

	if err != mongo.ErrNoDocuments {
		writeAuthJSONError(w, http.StatusInternalServerError, "Database error", AuthErrorResponse{})
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Registration failed", AuthErrorResponse{})
		return
	}
	code, err := generateVerificationCode()
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Registration failed", AuthErrorResponse{})
		return
	}
	codeHash, err := hashVerificationCode(code)
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Registration failed", AuthErrorResponse{})
		return
	}
	if err := sendVerificationCodeEmail(email, code); err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Registration failed", AuthErrorResponse{})
		return
	}
	now := time.Now().UTC()
	emailVerified := false
	user := User{
		Email:                      email,
		PasswordHash:               string(hashedPassword),
		CreatedAt:                  now,
		EmailVerified:              &emailVerified,
		EmailVerificationCodeHash:  codeHash,
		EmailVerificationExpiresAt: now.Add(10 * time.Minute),
		EmailVerificationAttempts:  0,
		EmailVerificationSentAt:    now,
	}
	result, err := collection.InsertOne(ctx, user)
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Failed to create user", AuthErrorResponse{})
		return
	}
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		writeAuthJSONError(w, http.StatusInternalServerError, "Failed to parse user ID", AuthErrorResponse{})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(RegisterResponse{
		Message:              "verification code sent",
		ID:                   insertedID.Hex(),
		Email:                email,
		RequiresVerification: true,
	})
}
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		claims, err := getClaimsFromRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), authClaimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}
func getAuthClaims(r *http.Request) (*AuthClaims, bool) {
	claims, ok := r.Context().Value(authClaimsKey).(*AuthClaims)
	return claims, ok
}
func getClaimsFromRequest(r *http.Request) (*AuthClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return nil, fmt.Errorf("invalid authorization header")
	}
	tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
	if tokenString == "" {
		return nil, fmt.Errorf("missing token")
	}
	return parseJWT(tokenString)
}
func generateJWT(user User) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET is not set")
	}
	claims := AuthClaims{
		UserID: user.ID.Hex(),
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
func parseJWT(tokenString string) (*AuthClaims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is not set")
	}
	claims := &AuthClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := normalizeEmail(req.Email)
	password := req.Password

	if email == "" || password == "" {
		writeAuthJSONError(w, http.StatusBadRequest, "Email and password are required", AuthErrorResponse{})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := mongoClient.Database("fund_tracking").Collection("users")

	var user User
	err = collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)

	if err == mongo.ErrNoDocuments {
		writeAuthJSONError(w, http.StatusUnauthorized, "Invalid email or password", AuthErrorResponse{})
		return
	}

	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Database error", AuthErrorResponse{})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		writeAuthJSONError(w, http.StatusUnauthorized, "Invalid email or password", AuthErrorResponse{})
		return
	}

	if !isEmailVerifiedForLogin(user) {
		writeAuthJSONError(w, http.StatusForbidden, "Email verification required", AuthErrorResponse{
			RequiresVerification: true,
			Email:                user.Email,
		})
		return
	}

	token, err := generateJWT(user)
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Failed to generate token", AuthErrorResponse{})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	json.NewEncoder(w).Encode(LoginResponse{
		Message: "login successful",
		Token:   token,
		ID:      user.ID.Hex(),
		Email:   user.Email,
	})
}

type MeResponse struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := getAuthClaims(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	emailVerified := true
	if claims.UserID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if user, found, err := findUserByID(ctx, claims.UserID); err == nil && found {
			emailVerified = isEmailVerifiedForLogin(user)
		} else if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(MeResponse{
		UserID:        claims.UserID,
		Email:         claims.Email,
		EmailVerified: emailVerified,
	})
}

func verifyEmailCodeHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VerifyEmailCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthJSONError(w, http.StatusBadRequest, "Invalid or expired verification code", AuthErrorResponse{})
		return
	}
	email := normalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)
	if !isValidEmailFormat(email) || !isValidVerificationCodeFormat(code) {
		writeAuthJSONError(w, http.StatusBadRequest, "Invalid or expired verification code", AuthErrorResponse{})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collection := mongoClient.Database("fund_tracking").Collection("users")

	var user User
	err := collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		writeAuthJSONError(w, http.StatusBadRequest, "Invalid or expired verification code", AuthErrorResponse{})
		return
	}

	if isEmailExplicitlyVerified(user) {
		writeVerificationSuccess(w, email)
		return
	}

	if !canAttemptEmailVerification(user, time.Now().UTC()) {
		writeAuthJSONError(w, http.StatusBadRequest, "Invalid or expired verification code", AuthErrorResponse{})
		return
	}

	codeHash, err := hashVerificationCode(code)
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Verification failed", AuthErrorResponse{})
		return
	}
	if codeHash != user.EmailVerificationCodeHash {
		_, _ = collection.UpdateOne(ctx, bson.M{"email": email}, bson.M{
			"$inc": bson.M{"emailVerificationAttempts": 1},
		})
		writeAuthJSONError(w, http.StatusBadRequest, "Invalid or expired verification code", AuthErrorResponse{})
		return
	}

	now := time.Now().UTC()
	_, err = collection.UpdateOne(ctx, bson.M{"email": email}, bson.M{
		"$set": bson.M{
			"emailVerified":             true,
			"emailVerifiedAt":           now,
			"emailVerificationAttempts": 0,
		},
		"$unset": bson.M{
			"emailVerificationCodeHash":  "",
			"emailVerificationExpiresAt": "",
		},
	})
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Verification failed", AuthErrorResponse{})
		return
	}
	writeVerificationSuccess(w, email)
}

func resendEmailCodeHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResendEmailCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthJSONError(w, http.StatusBadRequest, "Invalid request body", AuthErrorResponse{})
		return
	}
	email := normalizeEmail(req.Email)
	if !isValidEmailFormat(email) {
		writeResendSuccess(w)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collection := mongoClient.Database("fund_tracking").Collection("users")

	var user User
	err := collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		writeResendSuccess(w)
		return
	}
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Resend failed", AuthErrorResponse{})
		return
	}
	if isEmailExplicitlyVerified(user) {
		writeResendSuccess(w)
		return
	}

	if retryAfter, ok := emailVerificationCooldownRemaining(user.EmailVerificationSentAt, time.Now().UTC()); ok {
		writeAuthJSONError(w, http.StatusTooManyRequests, "Please wait before requesting another code", AuthErrorResponse{
			RetryAfterSeconds: retryAfter,
		})
		return
	}

	code, err := generateVerificationCode()
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Resend failed", AuthErrorResponse{})
		return
	}
	codeHash, err := hashVerificationCode(code)
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Resend failed", AuthErrorResponse{})
		return
	}
	if err := sendVerificationCodeEmail(email, code); err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Resend failed", AuthErrorResponse{})
		return
	}

	now := time.Now().UTC()
	_, err = collection.UpdateOne(ctx, bson.M{"email": email}, bson.M{
		"$set": bson.M{
			"emailVerificationCodeHash":  codeHash,
			"emailVerificationExpiresAt": now.Add(10 * time.Minute),
			"emailVerificationAttempts":  0,
			"emailVerificationSentAt":    now,
		},
	})
	if err != nil {
		writeAuthJSONError(w, http.StatusInternalServerError, "Resend failed", AuthErrorResponse{})
		return
	}
	writeResendSuccess(w)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isValidEmailFormat(email string) bool {
	if email == "" {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(address.Address, email)
}

func isEmailVerifiedForLogin(user User) bool {
	return user.EmailVerified == nil || *user.EmailVerified
}

func isEmailExplicitlyVerified(user User) bool {
	return user.EmailVerified != nil && *user.EmailVerified
}

func canAttemptEmailVerification(user User, now time.Time) bool {
	if user.EmailVerificationAttempts >= 5 {
		return false
	}
	if strings.TrimSpace(user.EmailVerificationCodeHash) == "" {
		return false
	}
	if user.EmailVerificationExpiresAt.IsZero() {
		return false
	}
	return !now.After(user.EmailVerificationExpiresAt)
}

func emailVerificationCooldownRemaining(sentAt time.Time, now time.Time) (int, bool) {
	if sentAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(sentAt)
	if elapsed >= time.Minute {
		return 0, false
	}
	return int((time.Minute - elapsed).Seconds()), true
}

func findUserByID(ctx context.Context, userID string) (User, bool, error) {
	collection := mongoClient.Database("fund_tracking").Collection("users")
	filters := make([]bson.M, 0, 2)
	if objectID, err := primitive.ObjectIDFromHex(userID); err == nil {
		filters = append(filters, bson.M{"_id": objectID})
	}
	filters = append(filters, bson.M{"_id": userID})
	for _, filter := range filters {
		var user User
		err := collection.FindOne(ctx, filter).Decode(&user)
		if err == nil {
			return user, true, nil
		}
		if err != mongo.ErrNoDocuments {
			return User{}, false, err
		}
	}
	return User{}, false, nil
}

func writeVerificationSuccess(w http.ResponseWriter, email string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(VerifyEmailCodeResponse{
		Status:        "success",
		Message:       "email verified",
		Email:         email,
		EmailVerified: true,
	})
}

func writeResendSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(ResendEmailCodeResponse{
		Status:  "success",
		Message: "verification code sent",
	})
}

func writeAuthJSONError(w http.ResponseWriter, statusCode int, message string, response AuthErrorResponse) {
	response.Error = message
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
