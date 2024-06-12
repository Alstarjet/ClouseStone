package utilities

import (
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(payload models.User, deviceid, secretKey string) (string, time.Time, error) {
	environment := os.Getenv("ENVIRONMENT")
	var exp time.Time
	if environment == "local" {
		exp = time.Now().Add(time.Minute * 2)
	} else {
		exp = time.Now().Add(time.Hour * 24 * 22)
	}
	claims := jwt.MapClaims{
		"exp": exp.Unix(),
	}

	claims["email"] = payload.Email
	claims["name"] = payload.Name
	claims["userid"] = payload.ID
	claims["typeclient"] = payload.TypeClient
	claims["deviceid"] = deviceid

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret key
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", exp, err
	}

	return tokenString, exp, nil
}
func GenerateRefreshToken(payload models.User, deviceid, secretKey string) (string, time.Time, error) {
	environment := os.Getenv("ENVIRONMENT")
	var exp time.Time
	if environment == "local" {
		exp = time.Now().Add(time.Minute * 3)
	} else {
		exp = time.Now().Add(time.Hour * 24 * 25)
	}
	claims := jwt.MapClaims{
		"exp": exp.Unix(),
	}

	claims["email"] = payload.Email
	claims["name"] = payload.Name
	claims["userid"] = payload.ID
	claims["typeclient"] = payload.TypeClient
	claims["deviceid"] = deviceid

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret key
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", exp, err
	}

	return tokenString, exp, nil
}
func DecodeToken(tokenString string, secretKey string) (jwt.MapClaims, error) {
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}
	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido")
	}

	return claims, nil
}
