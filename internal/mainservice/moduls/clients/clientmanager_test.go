package clients

import (
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	createat = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	updateat = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	// backupat lo pone el servidor; el del dispositivo se ignora
	backupat = time.Date(2026, 10, 30, 9, 5, 0, 0, time.UTC)
)

// Copia de un cliente como la tiene un dispositivo: todos los campos, aunque
// sólo haya cambiado alguno.
func deviceCopy(changed ...string) models.Client {
	return models.Client{
		UUID: "cli-1", Name: "Ana", Lastname: "López Ruiz", Age: 40, City: "Puebla",
		Neighborhood: "Centro", Address: "5 de Mayo 10", Phone: "2221234567", Daywork: "Lunes",
		CreateAt:      createat,
		UpdateAt:      updateat,
		BackupAt:      time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:        "active",
		ChangedFields: changed,
	}
}

// fullWithout es el cliente completo como se guarda en Mongo, sin los campos dados.
func fullWithout(fields ...string) bson.M {
	full := bson.M{
		"name": "Ana", "lastname": "López Ruiz", "age": int32(40), "city": "Puebla", "neighborhood": "Centro",
		"address": "5 de Mayo 10", "phone": "2221234567", "daywork": "Lunes",
		"createat": primitive.NewDateTimeFromTime(createat), "updateat": primitive.NewDateTimeFromTime(updateat),
		"status": "active",
	}
	for _, f := range fields {
		delete(full, f)
	}
	return full
}

// Lo que recibe Mongo por cada cliente del respaldo (un solo upsert).
func TestUpsert(t *testing.T) {
	blacklisted := deviceCopy("type")
	blacklisted.Type = "blacklist"
	withoutChanges := deviceCopy()
	withoutChanges.Type = "blacklist"

	tests := []struct {
		name   string
		client models.Client
		want   bson.M
	}{
		// Si ya existe, sólo cambia el apellido; si es nuevo, se guarda completo
		{"sólo el apellido", deviceCopy("lastname"),
			bson.M{"$set": bson.M{"lastname": "López Ruiz", "updateat": updateat, "backupat": backupat},
				"$setOnInsert": fullWithout("lastname", "updateat")}},
		{"mandar a lista negra", blacklisted,
			bson.M{"$set": bson.M{"type": "blacklist", "updateat": updateat, "backupat": backupat},
				"$setOnInsert": fullWithout("updateat")}},
		{"sacar de la lista negra (type vacío se quita)", deviceCopy("type"),
			bson.M{"$set": bson.M{"updateat": updateat, "backupat": backupat},
				"$setOnInsert": fullWithout("updateat"), "$unset": bson.M{"type": ""}}},
		{"varios campos", deviceCopy("name", "age", "phone", "status"),
			bson.M{"$set": bson.M{"name": "Ana", "age": int32(40), "phone": "2221234567", "status": "active",
				"updateat": updateat, "backupat": backupat},
				"$setOnInsert": fullWithout("name", "age", "phone", "status", "updateat")}},
		// Versiones anteriores de la app: el documento completo encima (y un type
		// vacío no se escribe, así que no saca a nadie de la lista negra)
		{"sin changedfields: el documento completo", deviceCopy(),
			bson.M{"$set": func() bson.M { m := fullWithout(); m["backupat"] = backupat; return m }()}},
		{"sin changedfields con lista negra", withoutChanges,
			bson.M{"$set": func() bson.M { m := fullWithout(); m["type"] = "blacklist"; m["backupat"] = backupat; return m }()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up, err := Upsert(tt.client)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if up.UUID != "cli-1" {
				t.Errorf("uuid = %q", up.UUID)
			}
			if got := up.Update(backupat); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("update = %v\nse esperaba  %v", got, tt.want)
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

// Lo que hace que un cliente nunca se pueda aceptar: el respaldo lo rechaza
// sólo a él.
func TestValidate(t *testing.T) {
	withPhone := func(phone any) models.Client { c := deviceCopy(); c.Phone = phone; return c }
	tests := []struct {
		name    string
		client  models.Client
		wantErr error
	}{
		{"sin changedfields", deviceCopy(), nil},
		{"todos los editables", deviceCopy("name", "lastname", "age", "city", "neighborhood", "address", "phone", "daywork", "status", "type"), nil},
		{"uuid no se cambia", deviceCopy("uuid"), ErrUnknownField},
		{"createat no se cambia", deviceCopy("createat"), ErrUnknownField},
		{"backupat lo pone el servidor", deviceCopy("backupat"), ErrUnknownField},
		{"usermongoid no se cambia", deviceCopy("lastname", "usermongoid"), ErrUnknownField},
		{"campo que no existe", deviceCopy("color"), ErrUnknownField},
		{"mayúsculas no", deviceCopy("Lastname"), ErrUnknownField},
		{"teléfono como número", withPhone(2221234567.0), nil},
		{"sin teléfono", withPhone(nil), nil},
		{"teléfono que es un objeto", withPhone(map[string]any{"$x": 1.0}), ErrPhoneType},
		{"teléfono que es una lista", withPhone([]any{"222"}), ErrPhoneType},
		{"teléfono verdadero/falso", withPhone(true), ErrPhoneType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.client)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("error inesperado: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v; se esperaba %v", err, tt.wantErr)
			}
		})
	}
}
