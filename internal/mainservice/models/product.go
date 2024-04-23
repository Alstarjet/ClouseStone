package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProductCartItem struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Page     string  `json:"page"`
	Price    float64 `json:"price"`
	Type     string  `json:"type"`
	Catalog  string  `json:"catalog"`
	Quantity int     `json:"quantity,omitempty"`
	Total    float64 `json:"total,omitempty"`
}

type Catalog struct {
	Catalog     string `json:"catalog"`
	Description string `json:"description"`
	Use         int    `json:"use"`
}

type Product struct {
	ID          primitive.ObjectID `json:"-" bson:"_id"`
	UUID        string             `json:"uuid"`
	Key         string             `json:"key"`
	Name        string             `json:"name"`
	Page        string             `json:"page"`
	Price       float64            `json:"price"`
	Type        string             `json:"type"`
	Catalog     string             `json:"catalog"`
	Quantity    int                `json:"quantity,omitempty"`
	Total       float64            `json:"total,omitempty"`
	Stock       float64            `json:"stock,omitempty"`
	UserMongoID string             `bson:"usermongoid" json:"-"`
	CreateAt    time.Time          `json:"createat"`
	UpdateAt    time.Time          `json:"updateat"`
	Status      string             `json:"status"`
}
