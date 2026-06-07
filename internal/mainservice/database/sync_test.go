package database

import (
	"financial-Assistant/internal/mainservice/models"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBuildSyncSetDoc(t *testing.T) {
	serverTime := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	const usermongoid = "user-mongo-id-123"

	t.Run("sella campos del servidor y descarta _id/createat", func(t *testing.T) {
		client := models.Client{
			ID:          primitive.NewObjectID(), // debe descartarse
			UUID:        "client-uuid-abc",
			Name:        "Juan",
			UserMongoID: "valor-falso-del-cliente",                   // debe sobrescribirse
			CreateAt:    time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), // debe descartarse
			UpdateAt:    time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC), // debe sobrescribirse
			Status:      StatusActive,
		}

		setDoc, err := buildSyncSetDoc(client, usermongoid, client.UUID, serverTime)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}

		if _, ok := setDoc["_id"]; ok {
			t.Error("_id no debería estar presente en el $set")
		}
		if _, ok := setDoc["createat"]; ok {
			t.Error("createat no debería estar en el $set (va en $setOnInsert)")
		}
		if got := setDoc["usermongoid"]; got != usermongoid {
			t.Errorf("usermongoid = %v; se esperaba %q", got, usermongoid)
		}
		if got := setDoc["uuid"]; got != client.UUID {
			t.Errorf("uuid = %v; se esperaba %q", got, client.UUID)
		}
		ut, ok := setDoc["updateat"].(time.Time)
		if !ok || !ut.Equal(serverTime) {
			t.Errorf("updateat = %v; se esperaba %v", setDoc["updateat"], serverTime)
		}
		if got := setDoc["name"]; got != "Juan" {
			t.Errorf("name = %v; se esperaba \"Juan\"", got)
		}
	})

	t.Run("status vacío se normaliza a active", func(t *testing.T) {
		client := models.Client{UUID: "u1", Status: ""}
		setDoc, err := buildSyncSetDoc(client, usermongoid, client.UUID, serverTime)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got := setDoc["status"]; got != StatusActive {
			t.Errorf("status = %v; se esperaba %q", got, StatusActive)
		}
	})

	t.Run("status deleted (tombstone) se preserva", func(t *testing.T) {
		charge := models.Charge{UUID: "c1", Status: StatusDeleted}
		setDoc, err := buildSyncSetDoc(charge, usermongoid, charge.UUID, serverTime)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got := setDoc["status"]; got != StatusDeleted {
			t.Errorf("status = %v; se esperaba %q", got, StatusDeleted)
		}
	})
}
