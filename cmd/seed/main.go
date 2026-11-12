// Command seed carga archivos de mongoexport (JSON array con extended JSON:
// $oid, $date) en una colección de MongoDB local. Es una utilidad de desarrollo
// para recrear data en local; no forma parte del servicio.
//
// Sólo corre contra un MongoDB local (localhost, 127.0.0.1 o ::1): con -drop
// borra la colección completa, así que nunca debe tocar producción (I-15).
//
// Uso:
//
//	go run ./cmd/seed -uri mongodb://localhost:27017 -db CloudStoneProd \
//	    -file "D:\Gluzie\CloudStoneProd.clients.json" -collection clients -drop
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"financial-Assistant/cmd/internal/localmongo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	uri := flags.String("uri", "mongodb://localhost:27017", "URI de conexión a MongoDB (sólo local)")
	db := flags.String("db", "", "Base de datos destino (requerido)")
	file := flags.String("file", "", "Ruta del archivo .json (mongoexport array) (requerido)")
	collection := flags.String("collection", "", "Colección destino (requerido)")
	drop := flags.Bool("drop", false, "Vaciar la colección antes de insertar")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *db == "" || *file == "" || *collection == "" {
		flags.Usage()
		return errors.New("-db, -file y -collection son obligatorios")
	}
	// Antes de leer el archivo o conectarse a nada
	if err := localmongo.RequireLocal(*uri); err != nil {
		return err
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("leer archivo: %w", err)
	}

	// El export es un array de extended JSON: $oid -> ObjectID, $date -> DateTime.
	var docs []bson.M
	if err := bson.UnmarshalExtJSON(data, false, &docs); err != nil {
		return fmt.Errorf("parsear extended JSON: %w", err)
	}
	if len(docs) == 0 {
		log.Printf("seed: %s no contiene documentos; nada que insertar", *file)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*uri))
	if err != nil {
		return fmt.Errorf("conectar: %w", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	coll := client.Database(*db).Collection(*collection)

	if *drop {
		if err := coll.Drop(ctx); err != nil {
			return fmt.Errorf("drop %s.%s: %w", *db, *collection, err)
		}
		log.Printf("seed: colección %s.%s vaciada", *db, *collection)
	}

	insert := make([]interface{}, len(docs))
	for i := range docs {
		insert[i] = docs[i]
	}

	res, err := coll.InsertMany(ctx, insert, options.InsertMany().SetOrdered(false))
	if err != nil {
		return fmt.Errorf("insertar en %s.%s: %w", *db, *collection, err)
	}

	fmt.Printf("OK: %d documentos insertados en %s.%s\n", len(res.InsertedIDs), *db, *collection)
	return nil
}
