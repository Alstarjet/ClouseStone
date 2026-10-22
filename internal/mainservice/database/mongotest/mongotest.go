// Package mongotest conecta las pruebas que necesitan MongoDB a una base
// desechable.
//
// Sólo corre si CLOUSESTONE_TEST_MONGO_URI apunta a un servidor de pruebas; si
// no, la prueba se salta y `go test ./...` pasa sin Mongo. Cada llamada a Connect
// crea una base nueva con el prefijo clousestone_pruebas_ y la borra al terminar.
// Nunca lee DATA_BASE_APPLY_URI ni DATA_BASE_MONGO del entorno: las pisa sólo
// durante la prueba.
package mongotest

import (
	"context"
	"financial-Assistant/internal/mainservice/database"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnvURI es la variable con la URI del servidor de pruebas (MongoDB 5.0 o más
// reciente: el cliente usa la Stable API v1).
const EnvURI = "CLOUSESTONE_TEST_MONGO_URI"

const dbPrefix = "clousestone_pruebas_"

// Connect devuelve un database.MongoClient sobre una base vacía y exclusiva de
// la prueba, o salta la prueba si no hay servidor de pruebas configurado.
func Connect(t *testing.T) *database.MongoClient {
	t.Helper()
	uri := os.Getenv(EnvURI)
	if uri == "" {
		t.Skipf("sin %s: se salta la prueba con MongoDB", EnvURI)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("mongotest: conectar: %v", err)
	}

	name := fmt.Sprintf("%s%d", dbPrefix, time.Now().UnixNano())
	existing, err := admin.ListDatabaseNames(ctx, bson.M{"name": name})
	if err != nil {
		t.Fatalf("mongotest: listar bases: %v", err)
	}
	if len(existing) > 0 {
		t.Fatalf("mongotest: la base %q ya existe; no se toca", name)
	}

	t.Setenv("DATA_BASE_APPLY_URI", uri)
	t.Setenv("DATA_BASE_MONGO", name)
	db, err := database.NewMongoClient()
	if err != nil {
		t.Fatalf("mongotest: NewMongoClient: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := db.Disconnect(); err != nil {
			t.Errorf("mongotest: desconectar: %v", err)
		}
		if err := admin.Database(name).Drop(ctx); err != nil {
			t.Errorf("mongotest: borrar la base %q: %v", name, err)
		}
		if err := admin.Disconnect(ctx); err != nil {
			t.Errorf("mongotest: desconectar: %v", err)
		}
	})
	return db
}
