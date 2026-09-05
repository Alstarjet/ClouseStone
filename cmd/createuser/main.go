// Command createuser crea (o reemplaza) un usuario en la colección `users` con
// la contraseña hasheada con bcrypt. Utilidad de desarrollo para poder hacer
// login en local contra data ya existente: el _id debe coincidir con el
// usermongoid de los documentos del usuario.
//
// Uso:
//
//	go run ./cmd/createuser -uri mongodb://localhost:27017 -db gluzie_local \
//	    -id 6559353b829b4f93b7916a75 -email test@gluzie.local -password gluzie1234 -name Test
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	uri := flag.String("uri", "mongodb://localhost:27017", "URI de conexión a MongoDB")
	db := flag.String("db", "", "Base de datos destino (requerido)")
	idHex := flag.String("id", "", "_id del usuario en hex (debe coincidir con usermongoid de la data)")
	email := flag.String("email", "", "Email de login (requerido)")
	password := flag.String("password", "", "Contraseña en claro (se hashea con bcrypt) (requerido)")
	name := flag.String("name", "Test", "Nombre")
	lastname := flag.String("lastname", "Local", "Apellido")
	typeClient := flag.String("typeclient", "Quartz", "Tipo de cliente")
	flag.Parse()

	if *db == "" || *email == "" || *password == "" {
		flag.Usage()
		log.Fatal("createuser: -db, -email y -password son obligatorios")
	}

	var oid primitive.ObjectID
	var err error
	if *idHex != "" {
		oid, err = primitive.ObjectIDFromHex(*idHex)
		if err != nil {
			log.Fatalf("createuser: -id inválido: %v", err)
		}
	} else {
		oid = primitive.NewObjectID()
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("createuser: bcrypt: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*uri))
	if err != nil {
		log.Fatalf("createuser: conectar: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("createuser: ping: %v", err)
	}

	doc := bson.M{
		"_id":        oid,
		"name":       *name,
		"lastname":   *lastname,
		"email":      *email,
		"age":        int64(0),
		"password":   string(hash),
		"phone":      int64(0),
		"typeclient": *typeClient,
	}

	coll := client.Database(*db).Collection("users")
	_, err = coll.ReplaceOne(ctx, bson.M{"_id": oid}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		log.Fatalf("createuser: upsert: %v", err)
	}

	log.Printf("OK: usuario %q creado/actualizado en %s.users (_id=%s, typeclient=%s)", *email, *db, oid.Hex(), *typeClient)
}
