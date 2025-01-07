package utilities

import (
	"financial-Assistant/internal/mainservice/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestJwtCreate(t *testing.T) {

	objectID, _ := primitive.ObjectIDFromHex("hexID")
	secretKey := "UnitTest343"
	client := models.User{
		ID:         objectID,
		Name:       "John",
		LastName:   "Doe",
		Email:      "johndoe@example.com",
		Password:   "password123",
		Phone:      1234567890,
		TypeClient: "premium",
	}
	token, _, _ := GenerateToken(client, "NotDivice", secretKey)
	decodToken, _ := DecodeToken(token, secretKey)

	result := decodToken["email"]
	expect := client.Email
	assert.Equal(t, expect, result, "After codify a JWT and decodify we need obtain a same email")
}
