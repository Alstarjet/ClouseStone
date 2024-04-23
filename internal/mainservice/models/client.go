package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ID           primitive.ObjectID `json:"-" bson:"_id"`
	UUID         string             `json:"uuid"`
	Name         string             `json:"name"`
	Lastname     string             `json:"lastname"`
	Age          int                `json:"age"`
	City         string             `json:"city"`
	Neighborhood string             `json:"neighborhood"`
	Address      string             `json:"address"`
	Phone        interface{}        `json:"phone"`
	Daywork      string             `json:"daywork"`
	UserMongoID  string             `bson:"usermongoid" json:"-"`
	CreateAt     time.Time          `json:"createat"`
	UpdateAt     time.Time          `json:"updateat"`
	Status       string             `json:"status"`
}
