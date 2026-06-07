package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Estados de un documento sincronizable.
const (
	StatusActive  = "active"
	StatusDeleted = "deleted"
)

// upsertSyncItem inserta o actualiza un documento identificándolo por
// (usermongoid, uuid). El servidor sella updateat con serverTime en cada
// escritura y createat solo en la inserción, de modo que el cursor de
// sincronización siempre proviene del reloj del servidor. Es idempotente: subir
// el mismo uuid dos veces actualiza el mismo documento, nunca lo duplica.
func (mc *MongoClient) upsertSyncItem(ctx context.Context, collection, usermongoid, uuid string, item any, serverTime time.Time) error {
	if uuid == "" {
		return fmt.Errorf("upsertSyncItem: uuid vacío en colección %q", collection)
	}

	setDoc, err := buildSyncSetDoc(item, usermongoid, uuid, serverTime)
	if err != nil {
		return fmt.Errorf("upsertSyncItem en %q: %w", collection, err)
	}

	filter := bson.D{
		{Key: "usermongoid", Value: usermongoid},
		{Key: "uuid", Value: uuid},
	}
	update := bson.D{
		{Key: "$set", Value: setDoc},
		{Key: "$setOnInsert", Value: bson.D{{Key: "createat", Value: serverTime}}},
	}
	opts := options.Update().SetUpsert(true)

	coll := mc.client.Database(DataBase).Collection(collection)
	if _, err := coll.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("upsertSyncItem: update en %q: %w", collection, err)
	}
	return nil
}

// buildSyncSetDoc convierte un documento del cliente en el $set del upsert,
// reemplazando los campos controlados por el servidor: descarta _id (lo asigna
// Mongo) y createat (se preserva o se sella en $setOnInsert), y fija updateat,
// usermongoid, uuid y un status por defecto "active" si viene vacío.
func buildSyncSetDoc(item any, usermongoid, uuid string, serverTime time.Time) (bson.M, error) {
	data, err := bson.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var setDoc bson.M
	if err := bson.Unmarshal(data, &setDoc); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	delete(setDoc, "_id")
	delete(setDoc, "createat")
	setDoc["updateat"] = serverTime
	setDoc["usermongoid"] = usermongoid
	setDoc["uuid"] = uuid
	if s, _ := setDoc["status"].(string); s == "" {
		setDoc["status"] = StatusActive
	}
	return setDoc, nil
}

// findChangedSince devuelve los documentos de una colección que pertenecen al
// usuario y cambiaron después de since. Si since es nil (primera sincronización)
// devuelve el snapshot completo de registros activos. Si since no es nil incluye
// los tombstones (status="deleted") para que el cliente propague las
// eliminaciones.
func findChangedSince[T any](ctx context.Context, mc *MongoClient, collection, usermongoid string, since *time.Time) ([]T, error) {
	filter := bson.D{{Key: "usermongoid", Value: usermongoid}}
	if since != nil {
		filter = append(filter, bson.E{Key: "updateat", Value: bson.D{{Key: "$gt", Value: *since}}})
	} else {
		// Primera sincronización: solo registros activos (el cliente no necesita
		// tombstones porque parte de una base vacía).
		filter = append(filter, bson.E{Key: "status", Value: bson.D{{Key: "$ne", Value: StatusDeleted}}})
	}

	coll := mc.client.Database(DataBase).Collection(collection)
	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("findChangedSince: find en %q: %w", collection, err)
	}
	defer cursor.Close(ctx)

	results := make([]T, 0)
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("findChangedSince: decode en %q: %w", collection, err)
	}
	return results, nil
}

// --- Wrappers tipados (push) ---------------------------------------------------

func (mc *MongoClient) UpsertClientSync(ctx context.Context, usermongoid string, c models.Client, serverTime time.Time) error {
	return mc.upsertSyncItem(ctx, clients, usermongoid, c.UUID, c, serverTime)
}

func (mc *MongoClient) UpsertChargeSync(ctx context.Context, usermongoid string, c models.Charge, serverTime time.Time) error {
	return mc.upsertSyncItem(ctx, charges, usermongoid, c.UUID, c, serverTime)
}

func (mc *MongoClient) UpsertOrderSync(ctx context.Context, usermongoid string, o models.Charge, serverTime time.Time) error {
	return mc.upsertSyncItem(ctx, orders, usermongoid, o.UUID, o, serverTime)
}

func (mc *MongoClient) UpsertPaymentSync(ctx context.Context, usermongoid string, p models.Payment, serverTime time.Time) error {
	return mc.upsertSyncItem(ctx, payments, usermongoid, p.UUID, p, serverTime)
}

func (mc *MongoClient) UpsertProductSync(ctx context.Context, usermongoid string, p models.Product, serverTime time.Time) error {
	return mc.upsertSyncItem(ctx, products, usermongoid, p.UUID, p, serverTime)
}

// --- Wrappers tipados (pull) ---------------------------------------------------

func (mc *MongoClient) FindClientsChangedSince(ctx context.Context, usermongoid string, since *time.Time) ([]models.Client, error) {
	return findChangedSince[models.Client](ctx, mc, clients, usermongoid, since)
}

func (mc *MongoClient) FindChargesChangedSince(ctx context.Context, usermongoid string, since *time.Time) ([]models.Charge, error) {
	return findChangedSince[models.Charge](ctx, mc, charges, usermongoid, since)
}

func (mc *MongoClient) FindOrdersChangedSince(ctx context.Context, usermongoid string, since *time.Time) ([]models.Charge, error) {
	return findChangedSince[models.Charge](ctx, mc, orders, usermongoid, since)
}

func (mc *MongoClient) FindPaymentsChangedSince(ctx context.Context, usermongoid string, since *time.Time) ([]models.Payment, error) {
	return findChangedSince[models.Payment](ctx, mc, payments, usermongoid, since)
}

func (mc *MongoClient) FindProductsChangedSince(ctx context.Context, usermongoid string, since *time.Time) ([]models.Product, error) {
	return findChangedSince[models.Product](ctx, mc, products, usermongoid, since)
}

// --- Índices -------------------------------------------------------------------

// EnsureSyncIndexes crea los índices que el sync v2 necesita en cada colección:
//   - { usermongoid:1, updateat:1 }  → consulta de delta eficiente (obligatorio).
//   - { usermongoid:1, uuid:1 } único → refuerza la idempotencia del upsert.
//
// La creación es idempotente (no hace nada si el índice ya existe). El índice
// único se intenta en modo best-effort: si la data existente tuviera duplicados
// históricos, se registra el error y se continúa, sin bloquear el arranque.
func (mc *MongoClient) EnsureSyncIndexes(ctx context.Context) error {
	syncCollections := []string{clients, charges, orders, payments, products}

	deltaIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "usermongoid", Value: 1}, {Key: "updateat", Value: 1}},
	}
	uniqueIdentityIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "usermongoid", Value: 1}, {Key: "uuid", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("usermongoid_uuid_unique"),
	}

	for _, name := range syncCollections {
		coll := mc.client.Database(DataBase).Collection(name)

		if _, err := coll.Indexes().CreateOne(ctx, deltaIndex); err != nil {
			return fmt.Errorf("EnsureSyncIndexes: índice delta en %q: %w", name, err)
		}

		if _, err := coll.Indexes().CreateOne(ctx, uniqueIdentityIndex); err != nil {
			// No fatal: puede fallar si existe data duplicada previa. Se reporta
			// para limpiarla manualmente, pero el sync funciona sin el índice único.
			log.Printf("EnsureSyncIndexes: índice único en %q no creado (revisar duplicados): %v", name, err)
		}
	}
	return nil
}
