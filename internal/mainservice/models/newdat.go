package models

import "time"

type Payment struct {
	ClientUUID string    `json:"clientuuid"`
	ClientName string    `json:"clientname"`
	UUID       string    `json:"uuid"`
	Amount     int       `json:"amount"`
	Method     string    `json:"method"`
	Concept    string    `json:"concept"`
	Date       time.Time `json:"date"`
}

type Charge struct {
	ClientUUID string            `json:"clientuuid"`
	ClientName string            `json:"clientname"`
	UUID       string            `json:"uuid"`
	Products   []ProductCartItem `json:"products"`
	Discount   int               `json:"discount"`
	Subtotal   int               `json:"subtotal"`
	FinalPrice int               `json:"finalprice"`
	Date       time.Time         `json:"date"`
}

type ProductCartItem struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Page     string `json:"page"`
	Price    int    `json:"price"`
	Type     string `json:"type"`
	Catalog  string `json:"catalog"`
	Quantity int    `json:"quantity,omitempty"`
	Total    int    `json:"total,omitempty"`
}

type Catalog struct {
	Catalog     string `json:"catalog"`
	Description string `json:"description"`
	Use         int    `json:"use"`
}

type Product struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Page        string `json:"page"`
	Price       int    `json:"price"`
	Type        string `json:"type"`
	Catalog     string `json:"catalog"`
	Quantity    int    `json:"quantity,omitempty"`
	Total       int    `json:"total,omitempty"`
	Stock       int    `json:"stock,omitempty"`
	ProductUUID string `json:"productuuid"`
}
