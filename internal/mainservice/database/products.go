package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"log"

	"go.mongodb.org/mongo-driver/bson"
)

func (mc *MongoClient) AddProduct(user *models.Product) (interface{}, error) {
	collection := mc.client.Database("Agenda-StoneMoon").Collection("products")
	req, err := collection.InsertOne(context.Background(), user)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) FindProduct(key string) (models.Product, error) {
	filter := bson.M{"key": key}
	collection := mc.client.Database("Agenda-StoneMoon").Collection("products")
	var reques models.Product
	err := collection.FindOne(context.Background(), filter).Decode(&reques)
	if err != nil {
		log.Println(err)
	}
	return reques, err
}
func (mc *MongoClient) FindAllProducts() ([]models.Product, error) {
	collection := mc.client.Database("Agenda-StoneMoon").Collection("products")
	var products []models.Product
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer cursor.Close(context.Background())
	if err := cursor.All(context.Background(), &products); err != nil {
		log.Println(err)
		return nil, err
	}
	return products, nil
}
