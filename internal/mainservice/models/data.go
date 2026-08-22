package models

import "time"

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
	Toke       string    `json:"token"`
	Expires    time.Time `json:"expires"`
	UserName   string    `json:"username"`
	Data       AllData   `json:"data"`
	TypeClient string    `json:"typeclient"`
	// RefreshToken viaja en el cuerpo para clientes que no pueden usar la
	// cookie (cross-site o app nativa); lo envían de vuelta en el header
	// X-Refresh-Token. En /GetJwt solo se incluye cuando el token rotó.
	RefreshToken string `json:"refreshtoken,omitempty"`
}
