package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	ID       primitive.ObjectID `bson:"_id"`
	Name     string             `bson:"name" json:"name"`
	LastName string             `bson:"lastname" json:"lastname"`
	Email    string             `bson:"email" json:"email"`
	Password string             `bson:"password" json:"password"`
	Phone    int64              `bson:"phone" json:"phone"`
}
