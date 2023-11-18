package models

type ClientRegister struct {
	Name       string `json:"name"`
	ClientUuid string `json:"clientuuid"`
	Address    string `json:"address"`
	Phone      int64  `json:"phone"`
	DayWork    string `json:"daywork"`
	UserEmail  string
}
type ClientResponce struct {
	Name       string `json:"name"`
	ClientUuid string `json:"uuid"`
	Address    string `json:"address"`
	Phone      int64  `json:"phone"`
	DayWork    string `json:"daywork"`
}
