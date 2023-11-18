package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"log"

	"go.mongodb.org/mongo-driver/bson"
)

func (mc *MongoClient) AddClient(client models.ClientRegister) (interface{}, error) {
	collection := mc.client.Database(DataBase).Collection("clients")
	req, err := collection.InsertOne(context.Background(), client)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) FindClient(useremail string, clientuuid string, name string) (models.ClientRegister, error) {
	filter := bson.M{"useremail": useremail, "clientuuid": clientuuid, "name": name}
	collection := mc.client.Database(DataBase).Collection("clients")
	var reques models.ClientRegister
	err := collection.FindOne(context.Background(), filter).Decode(&reques)
	if err != nil {
		return reques, err
	}
	return reques, nil
}
