// Command createuser crea (o reemplaza) un usuario en la colección `users` con
// la contraseña hasheada con bcrypt. Utilidad de desarrollo para poder hacer
// login en local contra data ya existente: el _id debe coincidir con el
// usermongoid de los documentos del usuario.
//
// Sólo corre contra un MongoDB local (localhost, 127.0.0.1 o ::1): reemplaza al
// usuario completo, así que contra producción regresaría la cuenta a la versión
// gratis y le borraría su vigencia (I-15).
//
// Uso:
//
//	go run ./cmd/createuser -uri mongodb://localhost:27017 -db gluzie_local \
//	    -id 6559353b829b4f93b7916a75 -email test@gluzie.local -password gluzie1234 -name Test
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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("createuser: %v", err)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("createuser", flag.ContinueOnError)
	uri := flags.String("uri", "mongodb://localhost:27017", "URI de conexión a MongoDB (sólo local)")
	db := flags.String("db", "", "Base de datos destino (requerido)")
	idHex := flags.String("id", "", "_id del usuario en hex (debe coincidir con usermongoid de la data)")
	email := flags.String("email", "", "Email de login (requerido)")
	password := flags.String("password", "", "Contraseña en claro (se hashea con bcrypt) (requerido)")
	name := flags.String("name", "Test", "Nombre")
	lastname := flags.String("lastname", "Local", "Apellido")
	typeClient := flags.String("typeclient", "Quartz", "Tipo de cliente")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *db == "" || *email == "" || *password == "" {
		flags.Usage()
		return errors.New("-db, -email y -password son obligatorios")
	}
	// Antes de conectarse a nada
	if err := localmongo.RequireLocal(*uri); err != nil {
		return err
	}

	var oid primitive.ObjectID
	var err error
	if *idHex != "" {
		oid, err = primitive.ObjectIDFromHex(*idHex)
		if err != nil {
			return fmt.Errorf("-id inválido: %w", err)
		}
	} else {
		oid = primitive.NewObjectID()
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*uri))
	if err != nil {
		return fmt.Errorf("conectar: %w", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("ping: %w", err)
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
		return fmt.Errorf("upsert: %w", err)
	}

	log.Printf("OK: usuario %q creado/actualizado en %s.users (_id=%s, typeclient=%s)", *email, *db, oid.Hex(), *typeClient)
	return nil
}
