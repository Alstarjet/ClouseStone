package utilities

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/models"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJwtCreate(t *testing.T) {
	var client models.User
	secretKey := "UnitTest343"

	body, err := os.ReadFile("samples/user.json")
	assert.NoError(t, err)

	err = json.Unmarshal(body, &client)
	assert.NoError(t, err)

	token, _, err := GenerateToken(client, "NotDivice", secretKey)
	assert.NoError(t, err)

	decodToken, err := DecodeToken(token, secretKey)
	assert.NoError(t, err)

	result := decodToken["email"]
	expect := client.Email
	assert.Equal(t, expect, result, "After codify a JWT and decodify we need obtain a same email")
}
