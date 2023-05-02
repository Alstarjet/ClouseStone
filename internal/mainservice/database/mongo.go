package database

import (
	"context"
	"log"

	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoClient struct {
	client *mongo.Client
}

func NewMongoClient() (*MongoClient, error) {
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI("mongodb+srv://AlbertStar:e89hbwfk7LpOJYel@cluster0.qvrviuf.mongodb.net/?retryWrites=true&w=majority").SetServerAPIOptions(serverAPI)

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

func (mc *MongoClient) InsertUser(user *models.Request) (interface{}, error) {
	log.Println("Estamos en Insert")
	collection := mc.client.Database("Budget-AI").Collection("users")
	req, err := collection.InsertOne(context.Background(), user)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) RegisterUser(user *models.User) (interface{}, error) {
	log.Println("Estamos en Insert")
	collection := mc.client.Database("Agenda-StoneMoon").Collection("users")
	req, err := collection.InsertOne(context.Background(), user)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) FindUser(email string) (models.User, error) {
	filter := bson.M{"email": email}
	collection := mc.client.Database("Agenda-StoneMoon").Collection("users")
	var reques models.User
	err := collection.FindOne(context.Background(), filter).Decode(&reques)
	if err != nil {
		log.Println(err)
	}
	return reques, err
}
