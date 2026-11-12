package database

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Índices -------------------------------------------------------------------

// EnsureSyncIndexes crea los índices de las colecciones del respaldo:
//   - { usermongoid:1, updateat:1 } → consultas por usuario.
//   - { usermongoid:1, uuid:1 } único (usermongoid_uuid_unique) → un solo
//     documento por uuid de cada usuario. Con él, dos subidas a la vez del mismo
//     documento no lo duplican (ver UpsertBackup).
//
// La creación es idempotente (no hace nada si el índice ya existe). Cada índice
// se intenta aunque otro falle; el único es best-effort: si la data existente
// tiene duplicados, se registra en el log y el arranque sigue (hay que limpiarlos
// a mano; ver docs/revision-produccion.md).
func (mc *MongoClient) EnsureSyncIndexes(ctx context.Context) error {
	deltaIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "usermongoid", Value: 1}, {Key: "updateat", Value: 1}},
	}
	uniqueIdentityIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "usermongoid", Value: 1}, {Key: "uuid", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("usermongoid_uuid_unique"),
	}

	var errs []error
	for _, name := range []string{clients, charges, orders, payments} {
		coll := mc.client.Database(DataBase).Collection(name)

		if _, err := coll.Indexes().CreateOne(ctx, deltaIndex); err != nil {
			errs = append(errs, fmt.Errorf("EnsureSyncIndexes: índice por usuario en %q: %w", name, err))
		}

		if _, err := coll.Indexes().CreateOne(ctx, uniqueIdentityIndex); err != nil {
			// No fatal: puede fallar si existe data duplicada previa. Se reporta
			// para limpiarla manualmente; mientras tanto el upsert del respaldo
			// sigue sin duplicar en el día a día, pero dos subidas a la vez sí podrían.
			log.Printf("EnsureSyncIndexes: índice único en %q no creado (revisar duplicados): %v", name, err)
		}
	}
	return errors.Join(errs...)
}
