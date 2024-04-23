package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const orders = "orders"

func (mc *MongoClient) AddOrder(Order models.Charge) (interface{}, error) {
	Order.ID = primitive.NewObjectID()
	collection := mc.client.Database(DataBase).Collection(orders)
	req, err := collection.InsertOne(context.Background(), Order)
	return req.InsertedID, err
}
func (mc *MongoClient) FindOrder(filter interface{}) (models.Charge, error) {
	collection := mc.client.Database(DataBase).Collection(orders)
	var result models.Charge
	err := collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		return models.Charge{}, err
	}
	return result, nil
}

func (mc *MongoClient) UpdateOrder(filter interface{}, Order models.Charge) error {
	update := bson.M{
		"$set": Order,
	}
	collection := mc.client.Database(DataBase).Collection(orders)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}
func (mc *MongoClient) FindAllOrders(filter interface{}) ([]models.Charge, error) {
	collection := mc.client.Database(DataBase).Collection(orders)
	var results []models.Charge
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var orders models.Charge
		if err := cursor.Decode(&orders); err != nil {
			return nil, err
		}
		results = append(results, orders)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
