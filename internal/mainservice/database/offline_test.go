package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// offlineClient es un cliente que nunca se conectó: cada operación falla con
// "client is disconnected" sin tocar la red ni ninguna base. Sirve para probar
// los caminos de error sin MongoDB.
func offlineClient(t *testing.T) *MongoClient {
	t.Helper()
	// NewClient (sin Connect) no abre conexiones; Connect sí empezaría a buscar el servidor.
	c, err := mongo.NewClient(options.Client().ApplyURI("mongodb://127.0.0.1:1")) //nolint:staticcheck // a propósito: sin conexión
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	previous := DataBase
	DataBase = "clousestone_sin_conexion"
	t.Cleanup(func() { DataBase = previous })
	return &MongoClient{client: c}
}

// Si Mongo falla, cada función regresa el error: ninguna usa el resultado antes
// de revisarlo (I-08: RegisterUser leía InsertedID de un resultado nil y tronaba).
func TestDatabaseErrorsAreReturned(t *testing.T) {
	db := offlineClient(t)
	ctx := context.Background()
	userID := primitive.NewObjectID()
	filter := bson.D{{Key: "usermongoid", Value: userID.Hex()}}
	calls := map[string]func() error{
		"RegisterUser": func() error {
			_, err := db.RegisterUser(ctx, &models.User{Email: "ana@ejemplo.mx"})
			return err
		},
		"FindUser": func() error { _, err := db.FindUser(ctx, "ana@ejemplo.mx"); return err },
		"FindUserByID": func() error {
			_, err := db.FindUserByID(ctx, userID.Hex())
			return err
		},
		"AddDevice":  func() error { return db.AddDevice(ctx, models.UserDevices{ID: userID}) },
		"FindDevice": func() error { _, err := db.FindDevice(ctx, bson.D{{Key: "_id", Value: userID}}); return err },
		"UpdateDeviceRefreshToken": func() error {
			_, err := db.UpdateDeviceRefreshToken(ctx, userID, "tel-1", "hash", time.Now())
			return err
		},
		"AddNewDevice": func() error {
			_, err := db.AddNewDevice(ctx, userID, models.Device{UUID: "tel-1"})
			return err
		},
		"RemoveDeviceByUUID": func() error { return db.RemoveDeviceByUUID(ctx, userID, "tel-1") },
		"ClearDeviceIDs":     func() error { return db.ClearDeviceIDs(ctx, userID, "tel-1") },
		"UpsertBackup": func() error {
			return db.UpsertBackup(ctx, "clients", userID.Hex(), time.Now(), []Upsert{{UUID: "cli-1", Set: bson.M{"name": "Ana"}}})
		},
		"FindAllClients":  func() error { _, err := db.FindAllClients(ctx, filter); return err },
		"FindAllCharges":  func() error { _, err := db.FindAllCharges(ctx, filter); return err },
		"FindAllOrders":   func() error { _, err := db.FindAllOrders(ctx, filter); return err },
		"FindAllPayments": func() error { _, err := db.FindAllPayments(ctx, filter); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("tronó en vez de regresar el error: %v", r)
				}
			}()
			if err := call(); err == nil {
				t.Fatal("sin conexión se esperaba un error")
			}
		})
	}
}

// w=majority salvo que la URI ya diga su write concern.
func TestWithMajority(t *testing.T) {
	tests := []struct {
		name, uri string
		wantW     any
	}{
		{"sin write concern en la URI", "mongodb://localhost:27017/", "majority"},
		{"otras opciones no cuentan", "mongodb://localhost:27017/?retryWrites=true&appName=gluzie", "majority"},
		{"la URI dice w=1: se respeta", "mongodb://localhost:27017/?w=1", 1},
		{"la URI ya dice majority", "mongodb://localhost:27017/?w=majority", "majority"},
		{"la URI sólo fija journal: se respeta tal cual", "mongodb://localhost:27017/?journal=true", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := withMajority(options.Client().ApplyURI(tt.uri))
			if opts.WriteConcern == nil {
				t.Fatalf("sin write concern")
			}
			if got := opts.WriteConcern.GetW(); !reflect.DeepEqual(got, tt.wantW) {
				t.Errorf("w = %v; se esperaba %v", got, tt.wantW)
			}
		})
	}
}

// Vaciar la cola de un dispositivo toca sólo sus cuatro listas: nunca la lista
// de dispositivos completa ni un refresh token (I-07).
func TestClearDeviceIDsUpdate(t *testing.T) {
	update, arrayFilters := clearDeviceIDsUpdate("tel-1")
	want := bson.D{{Key: "$set", Value: bson.D{
		{Key: "devices.$[d].clientids", Value: bson.A{}},
		{Key: "devices.$[d].chargeids", Value: bson.A{}},
		{Key: "devices.$[d].orderids", Value: bson.A{}},
		{Key: "devices.$[d].paymentids", Value: bson.A{}},
	}}}
	if !reflect.DeepEqual(update, want) {
		t.Errorf("update = %v; se esperaba %v", update, want)
	}
	wantFilters := []interface{}{bson.M{"d.uuid": "tel-1"}}
	if !reflect.DeepEqual(arrayFilters.Filters, wantFilters) {
		t.Errorf("filtros = %v; se esperaba %v", arrayFilters.Filters, wantFilters)
	}
}
