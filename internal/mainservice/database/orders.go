package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
)

const orders = "orders"

func (mc *MongoClient) FindAllOrders(ctx context.Context, filter interface{}) ([]models.Charge, error) {
	collection := mc.client.Database(DataBase).Collection(orders)
	var results []models.Charge
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var order models.Charge
		if err := cursor.Decode(&order); err != nil {
			return nil, err
		}
		results = append(results, order)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
