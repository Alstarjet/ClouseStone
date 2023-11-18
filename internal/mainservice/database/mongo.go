package database

import (
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoClient struct {
	client *mongo.Client
}

var DataBase = os.Getenv("DATA_BASE_MONGO")

func NewMongoClient() (*MongoClient, error) {
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(os.Getenv("DATA_BASE_APPLY_URI")).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	// Send a ping to confirm a successful connection
	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, err
	}
	log.Println("estamos en crear client", err)
	return &MongoClient{client: client}, nil
}

func (mc *MongoClient) Disconnect() error {
	return mc.client.Disconnect(context.Background())
}
