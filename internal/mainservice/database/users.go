package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (mc *MongoClient) RegisterUser(user *models.User) (interface{}, error) {
	user.ID = primitive.NewObjectID()
	user.TypeClient = "Quartz"
	collection := mc.client.Database(DataBase).Collection("users")
	req, err := collection.InsertOne(context.Background(), user)
	log.Println(req)
	return req.InsertedID, err
}
func (mc *MongoClient) FindUser(email string) (models.User, error) {
	filter := bson.M{"email": email}
	collection := mc.client.Database(DataBase).Collection("users")
	var reques models.User
	err := collection.FindOne(context.Background(), filter).Decode(&reques)
	if err != nil {
		log.Println(err)
	}
	return reques, err
}

func (mc *MongoClient) FindUserByID(id string) (models.User, error) {
	// Convert string ID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, fmt.Errorf("invalid user ID format: %v", err)
	}

	// Create a BSON filter for the user ID
	filter := bson.M{"_id": objectID}
	collection := mc.client.Database(DataBase).Collection("users")
	var user models.User

	// Find the user by ID
	err = collection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}
