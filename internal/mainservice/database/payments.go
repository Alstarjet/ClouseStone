package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const payments = "payments"

func (mc *MongoClient) AddPayment(Payment models.Payment) (interface{}, error) {
	Payment.ID = primitive.NewObjectID()
	collection := mc.client.Database(DataBase).Collection(payments)
	req, err := collection.InsertOne(context.Background(), Payment)
	return req.InsertedID, err
}
func (mc *MongoClient) FindPayment(filter interface{}) (models.Payment, error) {
	collection := mc.client.Database(DataBase).Collection(payments)
	var result models.Payment
	err := collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		return models.Payment{}, err
	}
	return result, nil
}

func (mc *MongoClient) UpdatePayment(filter interface{}, Payment models.Payment) error {
	update := bson.M{
		"$set": Payment,
	}
	collection := mc.client.Database(DataBase).Collection(payments)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}

func (mc *MongoClient) FindAllPayments(filter interface{}) ([]models.Payment, error) {
	collection := mc.client.Database(DataBase).Collection(payments)
	var results []models.Payment
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
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
