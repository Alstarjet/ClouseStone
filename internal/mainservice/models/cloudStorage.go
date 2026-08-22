package models

import "time"

type RequestUpdate struct {
	Clients  []Client  `json:"clients"`
	Payments []Payment `json:"payments"`
	Charges  []Charge  `json:"charges"`
	Orders   []Charge  `json:"orders"`
	Products []Product `json:"products"`
	DeviceID string    `json:"deviceid"`
}

type BackupResponse struct {
	Message    string `json:"message"`
	Status     int    `json:"reports"`
	TypeClient string `json:"typeclient"`
}

type AllData struct {
	Clients  []Client  `json:"clients"`
	Payments []Payment `json:"payments"`
	Charges  []Charge  `json:"charges"`
	Orders   []Charge  `json:"orders"`
}

// AllDataSince es la respuesta de GET /GetDataSince: el dataset (completo o
// delta por backupat) más la hora del servidor, que el cliente guarda —con su
// colchón— como cursor para la siguiente descarga.
type AllDataSince struct {
	AllData
	ServerTime time.Time `json:"servertime"`
}
