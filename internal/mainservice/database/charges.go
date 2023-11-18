package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const charges = "charges"

func (mc *MongoClient) AddMonthCharges(report *models.MonthCharges) (interface{}, error) {
	collection := mc.client.Database(DataBase).Collection(charges)
	req, err := collection.InsertOne(context.Background(), report)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) FindMonthCharges(useremail string, year int, month int) (models.MonthCharges, error) {
	filter := bson.M{"useremail": useremail, "year": year, "month": month}
	collection := mc.client.Database(DataBase).Collection(charges)
	var reques models.MonthCharges
	err := collection.FindOne(context.Background(), filter).Decode(&reques)
	if err != nil {
		return reques, err
	}
	return reques, nil
}
func (mc *MongoClient) UpdateMonthCharges(updatedReport *models.MonthCharges) (*mongo.UpdateResult, error) {
	collection := mc.client.Database(DataBase).Collection(charges)

	filter := bson.M{"_id": updatedReport.ID} // Filtrar por ID del reporte a actualizar

	update := bson.M{
		"$set": updatedReport, // Utiliza el operador $set para actualizar los campos
	}

	result, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return nil, err
	}

	return result, nil
}
