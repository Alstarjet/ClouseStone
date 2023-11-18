package models

type RequestUpload struct {
	Clients  []ClientRegister `json:"clients"`
	Payments []Payment        `json:"payments"`
	Charges  []Charge         `json:"charges"`
}
type BackupResponse struct {
	Message string `json:"message"`
	Status  int    `json:"reports"`
}
