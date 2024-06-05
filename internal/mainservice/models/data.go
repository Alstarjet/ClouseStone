package models

type JTW struct {
	Dato string `json:"dato"`
}
type DataLogin struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Device   string `json:"device"`
}
type DataJWT struct {
	UserMongoID string
	Email       string
	Name        string
	TypeClient  string
}

type JWTresponce struct {
	Toke         string  `json:"token"`
	Refreshtoken string  `json:"refreshtoken"`
	Hello        string  `json:"hello"`
	Data         AllData `json:"data"`
	NewDevice    bool    `json:"newdevice"`
	TypeClient   string  `json:"typeclient"`
}
