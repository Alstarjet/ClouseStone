package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MonthReport struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	Year      int                `bson:"year" json:"year"`
	Month     int                `bson:"month" json:"month"`
	UserEmail string             `bson:"useremail" json:"useremail"`
	LastDebt  float64            `bson:"lastdebt" json:"lastdebt"`
	Payments  []Payment          `bson:"payments" json:"payments"`
	Charges   []Charge           `bson:"charges" json:"charges"`
}
type Payment struct {
	ClientUuid string    `json:"clientuuid"`
	ClientName string    `json:"clientname"`
	Uuid       string    `json:"uuid"`
	Amount     float64   `json:"amount"`
	Method     string    `json:"method"`
	Concept    string    `json:"concept"`
	Date       time.Time `json:"date"`
}
type Charge struct {
	ClientUuid string    `json:"clientuuid"`
	ClientName string    `json:"clientname"`
	Uuid       string    `json:"uuid"`
	Products   []Product `json:"products"`
	Discount   float64   `json:"discount"`
	Subtotal   float64   `json:"subtotal"`
	FinalPrice float64   `json:"finalprice"`
	Date       time.Time `json:"date"`
}

type PaymentsForClient struct {
	ClientUuid string
	Payments   []Payment
}

type ChargesForClient struct {
	ClientUuid string
	Charges    []Charge
}

type PaymentsForMonth struct {
	Month    int
	Year     int
	Payments []Payment
}
type MonthPayments struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	Year      int                `bson:"year" json:"year"`
	Month     int                `bson:"month" json:"month"`
	UserEmail string             `bson:"useremail" json:"useremail"`
	Payments  []Payment          `bson:"payments" json:"payments"`
}
type MonthCharges struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	Year      int                `bson:"year" json:"year"`
	Month     int                `bson:"month" json:"month"`
	UserEmail string             `bson:"useremail" json:"useremail"`
	Charges   []Charge           `bson:"charges" json:"charges"`
}
