package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
)

const clients = "clients"

func (mc *MongoClient) FindAllClients(ctx context.Context, filter interface{}) ([]models.Client, error) {
	collection := mc.client.Database(DataBase).Collection(clients)
	var results []models.Client
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
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
