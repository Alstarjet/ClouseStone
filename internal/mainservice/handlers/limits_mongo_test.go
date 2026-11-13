package handlers

import (
	"context"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// Topes de S-05 con MongoDB (se saltan sin CLOUSESTONE_TEST_MONGO_URI, ver mongotest).

// Una cuenta conserva sólo los 20 dispositivos que entraron más recientemente:
// el documento de dispositivos ya no crece con cada login.
func TestLoginKeepsTheNewestDevices(t *testing.T) {
	db := mongotest.Connect(t)
	freshLoginLimits(t)
	user := registerWithPassword(t, db, "ana@ejemplo.mx")

	total := database.MaxDevicesPerAccount + 2
	for i := 1; i <= total; i++ {
		serveLogin(t, db, user.Email, fmt.Sprintf("dispositivo-%02d", i))
	}

	stored, _ := devicesDoc(t, db, user.ID)["devices"].(bson.A)
	if len(stored) != database.MaxDevicesPerAccount {
		t.Fatalf("quedaron %d dispositivos; se esperaban %d", len(stored), database.MaxDevicesPerAccount)
	}
	uuids := map[string]bool{}
	for _, d := range stored {
		device, _ := d.(bson.M)
		uuid, _ := device["uuid"].(string)
		uuids[uuid] = true
	}
	if !uuids[fmt.Sprintf("dispositivo-%02d", total)] {
		t.Errorf("el dispositivo que acaba de entrar no quedó: %v", uuids)
	}
	if uuids["dispositivo-01"] {
		t.Errorf("el dispositivo más viejo sigue ahí: %v", uuids)
	}

	// Volver a entrar con uno que ya está no agrega ni quita
	serveLogin(t, db, user.Email, fmt.Sprintf("dispositivo-%02d", total))
	if again, _ := devicesDoc(t, db, user.ID)["devices"].(bson.A); len(again) != database.MaxDevicesPerAccount {
		t.Errorf("después de repetir un dispositivo quedaron %d; se esperaban %d", len(again), database.MaxDevicesPerAccount)
	}
}

// CountBackup cuenta sólo los documentos de esa cuenta, y sólo en colecciones del
// respaldo.
func TestCountBackup(t *testing.T) {
	db := mongotest.Connect(t)
	ana := registerPaidUser(t, db, "ana@ejemplo.mx")
	rosa := registerPaidUser(t, db, "rosa@ejemplo.mx")
	mustUpload(t, db, ana.Email, validBackup)

	ctx := context.Background()
	for collection := range goodUUIDs {
		if n, err := db.CountBackup(ctx, collection, ana.ID.Hex()); err != nil || n != 1 {
			t.Errorf("%s de Ana: %d, %v; se esperaba 1", collection, n, err)
		}
		if n, err := db.CountBackup(ctx, collection, rosa.ID.Hex()); err != nil || n != 0 {
			t.Errorf("%s de Rosa: %d, %v; se esperaba 0", collection, n, err)
		}
	}
	if _, err := db.CountBackup(ctx, "users", ana.ID.Hex()); err == nil {
		t.Error("contar en users no dio error")
	}
	if _, err := db.CountBackup(ctx, "clients", ""); err == nil {
		t.Error("contar sin usuario no dio error")
	}
}
