package models

type User struct {
	Name     string `json:"name"`
	LastName string `json:"lastname"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    int64  `json:"phone"`
}
