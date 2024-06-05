package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const devices = "devices"

func (mc *MongoClient) AddDevice(Device models.UserDevices) (interface{}, error) {
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
	a, err := collection.UpdateOne(context.Background(), filter, update)
	fmt.Println(a)
	return err
}

// UpdateDeviceRefreshToken updates the refresh token of a specific device
func (mc *MongoClient) UpdateDeviceRefreshToken(userID string, deviceUUID string, newToken string, newExpiry time.Time) error {
	// Filtrar por el ID del usuario
	filter := bson.D{{Key: "_id", Value: userID}}

	// Definir la actualización del token de refresco
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "devices.$[elem].refreshtoken.token", Value: newToken},
			{Key: "devices.$[elem].refreshtoken.dateend", Value: newExpiry},
		}},
	}

	// Configurar los filtros de array para identificar el dispositivo específico
	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{bson.M{"elem.uuid": deviceUUID}},
	}
	updateOptions := options.UpdateOptions{
		ArrayFilters: &arrayFilters,
	}

	// Realizar la actualización en la colección 'userdevices'
	collection := mc.client.Database(DataBase).Collection(devices)
	o, err := collection.UpdateOne(context.Background(), filter, update, &updateOptions)
	fmt.Println(o)
	if err != nil {
		log.Println("Error updating refresh token:", err)
		return err
	}
	return nil
}
