package database

import (
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type MongoClient struct {
	client *mongo.Client
}

// DataBase es el nombre de la base de datos. Se asigna en NewMongoClient (no a
// nivel de paquete) para que se lea DESPUÉS de cargar el .env en main().
var DataBase string

func NewMongoClient() (*MongoClient, error) {
	DataBase = os.Getenv("DATA_BASE_MONGO")

	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := withMajority(options.Client().ApplyURI(os.Getenv("DATA_BASE_APPLY_URI")).SetServerAPIOptions(serverAPI))

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	// Send a ping to confirm a successful connection
	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, err
	}
	log.Println("MongoDB connected successfully")
	return &MongoClient{client: client}, nil
}

// withMajority pide que cada escritura la confirme la mayoría de los nodos
// (w=majority): un respaldo que respondió 200 no se pierde si cambia el
// servidor primario. Si la URI ya trae su propio write concern (w, journal o
// wtimeoutMS), se respeta tal cual.
func withMajority(opts *options.ClientOptions) *options.ClientOptions {
	if opts.WriteConcern == nil {
		opts.SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
	}
	return opts
}

func (mc *MongoClient) Disconnect() error {
	return mc.client.Disconnect(context.Background())
}
