package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Payment struct {
	ID          primitive.ObjectID `json:"-" bson:"_id"`
	UUID        string             `json:"uuid"`
	ClientUUID  string             `json:"clientuuid"`
	ClientName  string             `json:"clientname"`
	Amount      float64            `json:"amount"`
	Method      string             `json:"method"`
	Concept     string             `json:"concept"`
	UserMongoID string             `bson:"usermongoid" json:"-"`
	CreateAt    time.Time          `json:"createat"`
	UpdateAt    time.Time          `json:"updateat"`
	Status      string             `json:"status"`
}
