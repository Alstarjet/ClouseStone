// Command seed carga archivos de mongoexport (JSON array con extended JSON:
// $oid, $date) en una colección de MongoDB local. Es una utilidad de desarrollo
// para recrear data en local; no forma parte del servicio.
//
// Uso:
//
//	go run ./cmd/seed -uri mongodb://localhost:27017 -db CloudStoneProd \
//	    -file "D:\Gluzie\CloudStoneProd.clients.json" -collection clients -drop
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	uri := flag.String("uri", "mongodb://localhost:27017", "URI de conexión a MongoDB")
	db := flag.String("db", "", "Base de datos destino (requerido)")
	file := flag.String("file", "", "Ruta del archivo .json (mongoexport array) (requerido)")
	collection := flag.String("collection", "", "Colección destino (requerido)")
	drop := flag.Bool("drop", false, "Vaciar la colección antes de insertar")
	flag.Parse()

	if *db == "" || *file == "" || *collection == "" {
		flag.Usage()
		log.Fatal("seed: -db, -file y -collection son obligatorios")
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("seed: leer archivo: %v", err)
	}

	// El export es un array de extended JSON: $oid -> ObjectID, $date -> DateTime.
	var docs []bson.M
	if err := bson.UnmarshalExtJSON(data, false, &docs); err != nil {
		log.Fatalf("seed: parsear extended JSON: %v", err)
	}
	if len(docs) == 0 {
		log.Printf("seed: %s no contiene documentos; nada que insertar", *file)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*uri))
	if err != nil {
		log.Fatalf("seed: conectar: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("seed: ping: %v", err)
	}

	coll := client.Database(*db).Collection(*collection)

	if *drop {
		if err := coll.Drop(ctx); err != nil {
			log.Fatalf("seed: drop %s.%s: %v", *db, *collection, err)
		}
		log.Printf("seed: colección %s.%s vaciada", *db, *collection)
	}

	insert := make([]interface{}, len(docs))
	for i := range docs {
		insert[i] = docs[i]
	}

	res, err := coll.InsertMany(ctx, insert, options.InsertMany().SetOrdered(false))
	if err != nil {
		log.Fatalf("seed: insertar en %s.%s: %v", *db, *collection, err)
	}

	fmt.Printf("OK: %d documentos insertados en %s.%s\n", len(res.InsertedIDs), *db, *collection)
}
