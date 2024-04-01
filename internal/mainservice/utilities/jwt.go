package utilities

import (
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(payload models.DataJWT, secretKey string) (string, error) {
	// Set token claims
	claims := jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 150).Unix(),
	}

	claims["email"] = payload.Email
	claims["name"] = payload.Name

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret key
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
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
