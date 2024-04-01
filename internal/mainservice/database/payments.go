package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const payments = "payments"

func (mc *MongoClient) AddPayments(report *models.Payment) (interface{}, error) {
	collection := mc.client.Database(DataBase).Collection(payments)
	req, err := collection.InsertOne(context.Background(), report)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) FindPayments(useremail string, year int) (models.Payment, error) {
	filter := bson.M{"useremail": useremail, "year": year}
	collection := mc.client.Database(DataBase).Collection(payments)
	var reques models.Payment
	err := collection.FindOne(context.Background(), filter).Decode(&reques)
	if err != nil {
		return reques, err
	}
	return reques, nil
}
func (mc *MongoClient) UpdatePayments(updatedReport *models.Payment) (*mongo.UpdateResult, error) {
	collection := mc.client.Database(DataBase).Collection(payments)

	filter := bson.M{"_id": updatedReport.UUID} // Filtrar por ID del reporte a actualizar

	update := bson.M{
		"$set": updatedReport, // Utiliza el operador $set para actualizar los campos
	}

	result, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return nil, err
	}

	return result, nil
}
