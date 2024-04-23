package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const products = "products"

func (mc *MongoClient) AddProduct(Product models.Product) (interface{}, error) {
	Product.ID = primitive.NewObjectID()
	collection := mc.client.Database(DataBase).Collection(products)
	req, err := collection.InsertOne(context.Background(), Product)
	return req.InsertedID, err
}
func (mc *MongoClient) FindProduct(filter interface{}) (models.Product, error) {
	collection := mc.client.Database(DataBase).Collection(products)
	var result models.Product
	err := collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		return models.Product{}, err
	}
	return result, nil
}

func (mc *MongoClient) UpdateProduct(filter interface{}, Product models.Product) error {
	update := bson.M{
		"$set": Product,
	}
	collection := mc.client.Database(DataBase).Collection(products)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}
func (mc *MongoClient) FindAllProducts(filter interface{}) ([]models.Product, error) {
	collection := mc.client.Database(DataBase).Collection(products)
	var results []models.Product
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var product models.Product
		if err := cursor.Decode(&product); err != nil {
			return nil, err
		}
		results = append(results, product)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
