package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
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

func (mc *MongoClient) UpdateClient(filter interface{}, client models.Client) error {
	update := bson.M{
		"$set": client,
	}
	collection := mc.client.Database(DataBase).Collection(clients)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}
func (mc *MongoClient) FindAllClients(filter interface{}) ([]models.Client, error) {
	collection := mc.client.Database(DataBase).Collection(clients)
	var results []models.Client
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var client models.Client
		if err := cursor.Decode(&client); err != nil {
			return nil, err
		}
		results = append(results, client)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
