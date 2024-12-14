package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const charges = "charges"

func (mc *MongoClient) AddCharge(Charge models.Charge) (interface{}, error) {
	Charge.ID = primitive.NewObjectID()
	collection := mc.client.Database(DataBase).Collection(charges)
	req, err := collection.InsertOne(context.Background(), Charge)
	return req.InsertedID, err
}
func (mc *MongoClient) FindCharge(filter interface{}) (models.Charge, error) {
	collection := mc.client.Database(DataBase).Collection(charges)
	var result models.Charge
	err := collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		return models.Charge{}, err
	}
	return result, nil
}

func (mc *MongoClient) UpdateCharge(filter interface{}, Charge models.Charge) error {
	update := bson.M{
		"$set": Charge,
	}
	collection := mc.client.Database(DataBase).Collection(charges)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}

func (mc *MongoClient) FindAllCharges(filter interface{}) ([]models.Charge, error) {
	collection := mc.client.Database(DataBase).Collection(charges)
	var results []models.Charge
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
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
