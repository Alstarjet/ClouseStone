package models

type RequestUpload struct {
	Clients  []ClientRegister
	Payments []Payment
	Charges  []Charge
}
