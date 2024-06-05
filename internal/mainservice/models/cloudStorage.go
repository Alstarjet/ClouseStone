package models

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
