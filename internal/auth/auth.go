package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("error hashing user password")
		return "", err
	}
	return string(hashPass), nil
}

func CheckPasswordHash(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		log.Printf("hash and password don't match %v", err)
		return err
	}
	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string) (string, error) {
	hmacSecret := []byte(os.Getenv("SECRET"))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		Subject:   userID.String(),
	})

	tokenString, err := token.SignedString(hmacSecret)
	if err != nil {
		log.Printf("error signing token string %v", err)
		return "", err
	}
	return tokenString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		log.Printf("JWT parsing failed: %v", err)
		return uuid.UUID{}, err
	}
	userIDString, err := claims.GetSubject()
	if err != nil {
		log.Printf("error getting subject from claim: %v", err)
		return uuid.UUID{}, err
	}
	userID, err := uuid.Parse(userIDString)
	if err != nil {
		log.Printf("error converting uuid to string")
		return uuid.UUID{}, err
	}
	return userID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	if !strings.HasPrefix(headers.Get("Authorization"), "Bearer ") {
		log.Println("invalid auth header format given")
		return "", fmt.Errorf("invalid auth header")
	}
	token := strings.TrimPrefix(headers.Get("Authorization"), "Bearer ")

	return token, nil
}

func GetAPIToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")

	const prefix = "ApiKey "
	if !strings.HasPrefix(authHeader, prefix) {
		log.Println("invalid auth header format given")
		return "", fmt.Errorf("invalid auth header")
	}

	token := strings.TrimSpace(authHeader[len(prefix):])
	return token, nil
}

func MakeRefreshToken() (string, error) {
	randByte := make([]byte, 32)
	_, err := rand.Read(randByte)
	if err != nil {
		log.Printf("issue randomizing byte value %v", err)
		return "", err
	}
	refTokenString := hex.EncodeToString(randByte)
	return refTokenString, nil
}
