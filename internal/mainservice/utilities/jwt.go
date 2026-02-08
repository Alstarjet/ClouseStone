package utilities

import (
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
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
		exp = time.Now().Add(time.Minute * 15)
	}
	claims := jwt.MapClaims{
		"exp":        exp.Unix(),
		"iat":        time.Now().Unix(),
		"type":       "access",
		"email":      payload.Email,
		"name":       payload.Name,
		"userid":     payload.ID,
		"typeclient": payload.TypeClient,
		"deviceid":   deviceid,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

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
		exp = time.Now().Add(time.Hour * 24 * 30)
	}
	claims := jwt.MapClaims{
		"exp":      exp.Unix(),
		"iat":      time.Now().Unix(),
		"type":     "refresh",
		"userid":   payload.ID,
		"deviceid": deviceid,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", exp, err
	}

	return tokenString, exp, nil
}

func DecodeToken(tokenString string, secretKey string, expectedType string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritmo de firma inesperado: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido")
	}

	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != expectedType {
		return nil, fmt.Errorf("tipo de token inesperado: se esperaba %q", expectedType)
	}

	return claims, nil
}
