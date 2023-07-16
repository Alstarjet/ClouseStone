package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (mc *MongoClient) AddReport(report *models.MonthReport) (interface{}, error) {
	collection := mc.client.Database("Agenda-StoneMoon").Collection("reports")
	req, err := collection.InsertOne(context.Background(), report)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) FindReport(useremail string, clientuuid string, year int, month int) (models.MonthReport, error) {
	filter := bson.M{"useremail": useremail, "clientuuid": clientuuid, "year": year, "month": month}
	collection := mc.client.Database("Agenda-StoneMoon").Collection("reports")
	var reques models.MonthReport
	err := collection.FindOne(context.Background(), filter).Decode(&reques)
	if err != nil {
		return reques, err
	}
	return reques, nil
}
func (mc *MongoClient) UpdateReport(updatedReport *models.MonthReport) (*mongo.UpdateResult, error) {
	collection := mc.client.Database("Agenda-StoneMoon").Collection("reports")

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
