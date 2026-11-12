package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Lo que reciben Mongo y el BulkWrite del respaldo, sin conectarse.

var testBackupAt = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

func TestUpsertUpdate(t *testing.T) {
	tests := []struct {
		name string
		up   Upsert
		want bson.M
	}{
		{"documento completo: el servidor sella backupat",
			Upsert{UUID: "car-1", Set: bson.M{"finalprice": 301.0, "status": "active"}},
			bson.M{"$set": bson.M{"finalprice": 301.0, "status": "active", "backupat": testBackupAt}}},
		{"el dueño, el uuid, el _id y backupat nunca salen del dispositivo",
			Upsert{UUID: "car-1",
				Set:         bson.M{"usermongoid": "otro", "uuid": "otro", "_id": "x", "backupat": "1999", "total": 1.0},
				SetOnInsert: bson.M{"usermongoid": "otro", "createat": "c"},
				Unset:       bson.M{"uuid": "", "_id": ""}},
			bson.M{"$set": bson.M{"total": 1.0, "backupat": testBackupAt}, "$setOnInsert": bson.M{"createat": "c"}}},
		{"con campos sólo al crear y campos que se quitan",
			Upsert{UUID: "cli-1", Set: bson.M{"lastname": "López", "updateat": "u"},
				SetOnInsert: bson.M{"name": "Ana"}, Unset: bson.M{"type": ""}},
			bson.M{"$set": bson.M{"lastname": "López", "updateat": "u", "backupat": testBackupAt},
				"$setOnInsert": bson.M{"name": "Ana"}, "$unset": bson.M{"type": ""}}},
		// Mongo rechaza una escritura con el mismo campo en dos operadores: sería un
		// lote que nunca se guarda
		{"un campo repetido queda en uno solo",
			Upsert{UUID: "cli-1", Set: bson.M{"name": "Ana"},
				SetOnInsert: bson.M{"name": "Otra", "city": "Puebla"},
				Unset:       bson.M{"name": "", "city": "", "type": ""}},
			bson.M{"$set": bson.M{"name": "Ana", "backupat": testBackupAt},
				"$setOnInsert": bson.M{"city": "Puebla"}, "$unset": bson.M{"type": ""}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.up.Update(testBackupAt); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("update = %v; se esperaba %v", got, tt.want)
			}
		})
	}
}

// Cada documento es un upsert por (usermongoid, uuid), con la igualdad exacta del
// índice único (así Mongo reintenta solo el upsert que choca con otra subida).
func TestUpsertWrites(t *testing.T) {
	ups := []Upsert{
		{UUID: "cli-1", Set: bson.M{"name": "Ana"}},
		{UUID: "cli-2", Set: bson.M{"name": "Luis"}},
		{UUID: "cli-1", Set: bson.M{"name": "Ana María"}}, // el mismo, editado: va después
	}
	writes, err := upsertWrites("usuario-1", testBackupAt, ups)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(writes) != len(ups) {
		t.Fatalf("escrituras = %d; se esperaban %d", len(writes), len(ups))
	}
	for i, w := range writes {
		model, ok := w.(*mongo.UpdateOneModel)
		if !ok {
			t.Fatalf("escritura %d = %T; se esperaba un UpdateOneModel (con upsert)", i, w)
		}
		if model.Upsert == nil || !*model.Upsert {
			t.Errorf("escritura %d sin upsert: si el documento no existe no se guardaría", i)
		}
		wantFilter := bson.D{{Key: "usermongoid", Value: "usuario-1"}, {Key: "uuid", Value: ups[i].UUID}}
		if !reflect.DeepEqual(model.Filter, wantFilter) {
			t.Errorf("filtro %d = %v; se esperaba %v", i, model.Filter, wantFilter)
		}
		if want := ups[i].Update(testBackupAt); !reflect.DeepEqual(model.Update, want) {
			t.Errorf("update %d = %v; se esperaba %v", i, model.Update, want)
		}
	}

	if _, err := upsertWrites("usuario-1", testBackupAt, []Upsert{{Set: bson.M{"name": "Ana"}}}); err == nil {
		t.Error("un documento sin uuid se escribiría con uuid vacío")
	}
}

// Los campos del documento que se respaldan: todo lo del dispositivo, nada de lo
// que pone el servidor.
func TestDeviceFields(t *testing.T) {
	charge := models.Charge{
		ID: primitive.NewObjectID(), UUID: "car-1", ClientUUID: "cli-1", ClientName: "Ana López",
		Products: []models.ProductCartItem{
			{UUID: "prod-1", Key: "S1122", Name: "Crema", Page: "12", Price: 150.5, Type: "pieza", Catalog: "Shelo", Quantity: 2, Total: 301},
			{UUID: "prod-2", Key: "S2040", Name: "Regalo", Page: "30"}, // cantidad y total 0
		},
		FinalPrice:  301,
		UserMongoID: "valor-del-dispositivo",
		CreateAt:    time.Date(2026, 10, 20, 15, 5, 0, 0, time.UTC),
		BackupAt:    time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:      "active",
	}
	fields, err := DeviceFields(charge)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	for field := range serverFields {
		if _, ok := fields[field]; ok {
			t.Errorf("%q viene del dispositivo; lo pone el servidor", field)
		}
	}
	products, ok := fields["products"].(bson.A)
	if !ok || len(products) != 2 {
		t.Fatalf("products = %#v", fields["products"])
	}
	second, _ := products[1].(bson.M)
	if second["uuid"] != "prod-2" || second["quantity"] != int32(0) || second["total"] != 0.0 {
		t.Errorf("renglón con cantidad y total 0 = %v", second)
	}

	up, err := FullUpsert("car-1", charge)
	if err != nil || up.UUID != "car-1" || !reflect.DeepEqual(up.Set, fields) || up.SetOnInsert != nil || up.Unset != nil {
		t.Errorf("FullUpsert = %+v, %v; se esperaba el documento completo en Set", up, err)
	}
}

// UpsertBackup sólo escribe en las cuatro colecciones del respaldo, de un usuario,
// y un lote vacío no va a Mongo. Todo se decide antes de usar la conexión.
func TestUpsertBackupGuards(t *testing.T) {
	db := &MongoClient{} // sin conexión: si la usara, la prueba truena
	one := []Upsert{{UUID: "x", Set: bson.M{"a": 1}}}
	tests := []struct {
		name, collection, user string
		ups                    []Upsert
		wantErr                string
	}{
		{"otra colección", "users", "usuario-1", one, "no es una colección del respaldo"},
		{"los dispositivos tampoco", "devices", "usuario-1", one, "no es una colección del respaldo"},
		{"sin usuario", "clients", "", one, "falta el usuario"},
		{"lote vacío", "payments", "usuario-1", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.UpsertBackup(context.Background(), tt.collection, tt.user, testBackupAt, tt.ups)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("error inesperado: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v; se esperaba %q", err, tt.wantErr)
			}
		})
	}
}
