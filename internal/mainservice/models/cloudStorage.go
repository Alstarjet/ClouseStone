package models

import (
	"encoding/json"
	"time"
)

// RequestUpdate es el cuerpo de /UploadData. Cada registro llega crudo y el
// handler lo decodifica por separado: uno que no se puede aceptar (un tipo
// inesperado, sin uuid…) se rechaza solo, sin tumbar a los demás (I-02).
//
// El cuerpo también puede traer "products" y "deviceid": no se usan (los
// productos no se respaldan y el dispositivo sale del token), así que ni se
// leen y su forma no puede romper el respaldo.
type RequestUpdate struct {
	Clients  []json.RawMessage `json:"clients"`
	Charges  []json.RawMessage `json:"charges"`
	Orders   []json.RawMessage `json:"orders"`
	Payments []json.RawMessage `json:"payments"`
}

type BackupResponse struct {
	Message    string `json:"message"`
	Status     int    `json:"reports"`
	TypeClient string `json:"typeclient"`
}

// RejectedRecord es un registro del respaldo que el servidor nunca va a poder
// aceptar tal como viene. Store es la colección: "clients", "charges", "orders"
// o "payments"; UUID va vacío si el registro no traía uno; Reason lo explica en
// pocas palabras (es para leerse, no para que la app lo interprete).
type RejectedRecord struct {
	Store  string `json:"store"`
	UUID   string `json:"uuid"`
	Reason string `json:"reason"`
}

// BackupRejectedResponse es el cuerpo del 422 de /UploadData: todo lo demás del
// lote sí se guardó; sólo lo que está en Rejected no.
type BackupRejectedResponse struct {
	Error      string           `json:"error"` // siempre "rejected_records"
	Message    string           `json:"message"`
	TypeClient string           `json:"typeclient"`
	Rejected   []RejectedRecord `json:"rejected"`
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
