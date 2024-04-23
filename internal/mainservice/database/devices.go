package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const devices = "devices"

func (mc *MongoClient) AddDevice(Device models.UserDevices) (interface{}, error) {
	Device.ID = primitive.NewObjectID()
	collection := mc.client.Database(DataBase).Collection(devices)
	req, err := collection.InsertOne(context.Background(), Device)
	return req.InsertedID, err
}
func (mc *MongoClient) FindDevice(filter interface{}) (models.UserDevices, error) {
	collection := mc.client.Database(DataBase).Collection(devices)
	var result models.UserDevices
	err := collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		return models.UserDevices{}, err
	}
	return result, nil
}

func (mc *MongoClient) UpdateDevice(filter interface{}, Device models.UserDevices) error {
	update := bson.M{
		"$set": Device,
	}
	collection := mc.client.Database(DataBase).Collection(devices)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}
