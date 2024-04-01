package models

type JTW struct {
	Dato string `json:"dato"`
}
type DataLogin struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type DataJWT struct {
	UserMongoID string
	Email       string
	Name        string
}

type JWTresponce struct {
	Toke  string `json:"token"`
	Hello string `json:"hello"`
}
