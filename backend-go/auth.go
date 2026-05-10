package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

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
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}
type AuthClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}
type RegisterResponse struct {
	Message string `json:"message"`
	ID      string `json:"id"`
	Email   string `json:"email"`
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

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password
	if len(password) < 6 {
		http.Error(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := mongoClient.Database("fund_tracking").Collection("users")

	var existing User
	err = collection.FindOne(ctx, bson.M{"email": email}).Decode(&existing)

	if err == nil {
		http.Error(w, "Email already registered", http.StatusConflict)
		return
	}

	if err != mongo.ErrNoDocuments {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password ", http.StatusInternalServerError)
		return
	}
	user := User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
	}
	result, err := collection.InsertOne(ctx, user)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		http.Error(w, "Failed to parse user ID", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(RegisterResponse{
		Message: "registered",
		ID:      insertedID.Hex(),
		Email:   email,
	})
}
func generateJWT(user User) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", http.ErrNoCookie
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

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if email == "" || password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := mongoClient.Database("fund_tracking").Collection("users")

	var user User
	err = collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)

	if err == mongo.ErrNoDocuments {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	token, err := generateJWT(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
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
