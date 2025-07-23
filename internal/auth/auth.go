package auth

import (
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
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
		log.Printf("hash and password don't matchh %v", err)
		return err
	}
	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Println("error loading secret")
		return "", err
	}
	hmacSecret := []byte(os.Getenv("SECRET"))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
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
		log.Printf("token not valid %v", err)
		return uuid.Max, err
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		log.Printf("error converting uuid to string")
		return uuid.Max, err
	}
	return userID, nil
}
