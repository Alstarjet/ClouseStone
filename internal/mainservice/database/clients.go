package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const clients = "clients"

func (mc *MongoClient) AddClient(client models.Client) (interface{}, error) {
	client.ID = primitive.NewObjectID()
	collection := mc.client.Database(DataBase).Collection(clients)
	req, err := collection.InsertOne(context.Background(), client)
	return req.InsertedID, err
}
func (mc *MongoClient) FindClient(filter interface{}) (models.Client, error) {
	collection := mc.client.Database(DataBase).Collection(clients)
	var result models.Client
	err := collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		return models.Client{}, err
	}
	return result, nil
}

func (mc *MongoClient) UpdateClient(filter interface{}, update interface{}) error {
	collection := mc.client.Database(DataBase).Collection(clients)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}
