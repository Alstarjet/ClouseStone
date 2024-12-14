package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Charge struct {
	ID          primitive.ObjectID `bson:"_id" json:"-"`
	UUID        string             `json:"uuid"`
	ClientUUID  string             `json:"clientuuid"`
	ClientName  string             `json:"clientname"`
	Products    []ProductCartItem  `json:"products"`
	Discount    float64            `json:"discount"`
	Subtotal    float64            `json:"subtotal"`
	FinalPrice  float64            `json:"finalprice"`
	UserMongoID string             `bson:"usermongoid" json:"-"`
	CreateAt    time.Time          `json:"createat"`
	UpdateAt    time.Time          `json:"updateat"`
	Status      string             `json:"status"`
}
