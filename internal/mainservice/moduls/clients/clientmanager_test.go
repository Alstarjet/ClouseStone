package clients

import (
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Copia de un cliente como la tiene un dispositivo: todos los campos, aunque
// sólo haya cambiado alguno.
func deviceCopy(changed ...string) models.Client {
	return models.Client{
		UUID: "cli-1", Name: "Ana", Lastname: "López Ruiz", Age: 40, City: "Puebla",
		Neighborhood: "Centro", Address: "5 de Mayo 10", Phone: "2221234567", Daywork: "Lunes",
		CreateAt:      time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		UpdateAt:      time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC),
		BackupAt:      time.Date(2026, 10, 30, 9, 5, 0, 0, time.UTC),
		Status:        "active",
		ChangedFields: changed,
	}
}

func TestChangedFieldsUpdate(t *testing.T) {
	// updateat es del dispositivo y backupat del servidor: siempre se ponen
	updateat := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	backupat := time.Date(2026, 10, 30, 9, 5, 0, 0, time.UTC)
	blacklisted := deviceCopy("type")
	blacklisted.Type = "blacklist"

	tests := []struct {
		name   string
		client models.Client
		want   bson.M
	}{
		{"sólo el apellido", deviceCopy("lastname"),
			bson.M{"$set": bson.M{"lastname": "López Ruiz", "updateat": updateat, "backupat": backupat}}},
		{"mandar a lista negra", blacklisted,
			bson.M{"$set": bson.M{"type": "blacklist", "updateat": updateat, "backupat": backupat}}},
		{"sacar de la lista negra (type vacío se quita)", deviceCopy("type"),
			bson.M{"$set": bson.M{"updateat": updateat, "backupat": backupat}, "$unset": bson.M{"type": ""}}},
		{"varios campos", deviceCopy("name", "age", "phone", "status"),
			bson.M{"$set": bson.M{"name": "Ana", "age": int32(40), "phone": "2221234567", "status": "active",
				"updateat": updateat, "backupat": backupat}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changedFieldsUpdate(tt.client)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("update = %v; se esperaba %v", got, tt.want)
			}
		})
	}
}

// En Mongo nunca se guardan changedfields ni un type vacío (un respaldo sin el
// campo no borra la lista negra).
func TestClientBSON(t *testing.T) {
	tests := []struct {
		name     string
		client   models.Client
		wantType any
	}{
		{"sin type", deviceCopy("lastname"), nil},
		{"con type", func() models.Client { c := deviceCopy("type"); c.Type = "blacklist"; return c }(), "blacklist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := bson.Marshal(tt.client)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var doc bson.M
			if err := bson.Unmarshal(data, &doc); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if _, ok := doc["changedfields"]; ok {
				t.Errorf("se guardaría changedfields: %v", doc)
			}
			if doc["type"] != tt.wantType {
				t.Errorf("type = %v; se esperaba %v", doc["type"], tt.wantType)
			}
		})
	}
}

func TestValidateChangedFields(t *testing.T) {
	tests := []struct {
		name    string
		changed []string
		wantErr bool
	}{
		{"sin changedfields", nil, false},
		{"todos los editables", []string{"name", "lastname", "age", "city", "neighborhood", "address", "phone", "daywork", "status", "type"}, false},
		{"uuid no se cambia", []string{"uuid"}, true},
		{"createat no se cambia", []string{"createat"}, true},
		{"backupat lo pone el servidor", []string{"backupat"}, true},
		{"usermongoid no se cambia", []string{"lastname", "usermongoid"}, true},
		{"campo que no existe", []string{"color"}, true},
		{"mayúsculas no", []string{"Lastname"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok := deviceCopy()
			err := ValidateChangedFields([]models.Client{ok, deviceCopy(tt.changed...)})
			if tt.wantErr != (err != nil) {
				t.Fatalf("error = %v; se esperaba error: %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrUnknownField) {
				t.Errorf("error = %v; se esperaba ErrUnknownField", err)
			}
		})
	}
}
