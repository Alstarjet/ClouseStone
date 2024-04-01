package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ID           primitive.ObjectID `json:"_id" bson:"_id"`
	Name         string             `json:"name"`
	Lastname     string             `json:"lastname"`
	Age          int                `json:"age"`
	City         string             `json:"city"`
	Neighborhood string             `json:"neighborhood"`
	Address      string             `json:"address"`
	Phone        string             `json:"phone"`
	Daywork      string             `json:"daywork"`
	ClientUUID   string             `json:"clientuuid"`
	UserMongoID  string             `json:"usermongoid"`
	CreateAt     time.Time          `json:"createat"`
	UpdateAt     time.Time          `json:"updateat"`
}
