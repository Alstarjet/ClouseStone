package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
)

const charges = "charges"

func (mc *MongoClient) FindAllCharges(ctx context.Context, filter interface{}) ([]models.Charge, error) {
	collection := mc.client.Database(DataBase).Collection(charges)
	var results []models.Charge
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var charge models.Charge
		if err := cursor.Decode(&charge); err != nil {
			return nil, err
		}
		results = append(results, charge)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
