package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
)

const payments = "payments"

func (mc *MongoClient) FindAllPayments(ctx context.Context, filter interface{}) ([]models.Payment, error) {
	collection := mc.client.Database(DataBase).Collection(payments)
	var results []models.Payment
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var payment models.Payment
		if err := cursor.Decode(&payment); err != nil {
			return nil, err
		}
		results = append(results, payment)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
