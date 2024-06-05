package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID         primitive.ObjectID `bson:"_id"`
	Name       string             `bson:"name" json:"name"`
	LastName   string             `bson:"lastname" json:"lastname"`
	Email      string             `bson:"email" json:"email"`
	Password   string             `bson:"password" json:"password"`
	Phone      int64              `bson:"phone" json:"phone"`
	TypeClient string             `bson:"typeclient"`
}
type UserDevices struct {
	ID          primitive.ObjectID `bson:"_id"`
	UserMongoID string             `bson:"usermongoid"`
	UserName    string             `bson:"username"`
	UserEmail   string             `bson:"useremail"`
	Devices     []Device           `bson:"devices"`
}
type Device struct {
	UUID         string       `bson:"uuid"`
	ChargeIDs    []string     `bson:"chargeids"`
	PaymentIDs   []string     `bson:"paymentids"`
	ClientIDs    []string     `bson:"clientids"`
	OrderIDs     []string     `bson:"orderids"`
	Refreshtoken Refreshtoken `bson:"refreshtoken"`
}
type Refreshtoken struct {
	Token   string    `bson:"token"`
	DateEnd time.Time `bson:"dateend"`
}
