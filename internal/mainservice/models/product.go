package models

type Product struct {
	Key   string  `json:"key"`
	Price float64 `json:"price"`
	Name  string  `json:"name"`
	Page  int     `json:"page"`
	Year  int     `json:"year"`
}