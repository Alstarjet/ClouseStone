package mainservice

import (
	"financial-Assistant/internal/mainservice/database"
)

type Server struct {
	mongoDB *database.MongoClient
}

// NewRouter crea un nuevo router Gorilla Mux y configura sus rutas.
func NewServer() *Server {
	client, err := database.NewMongoClient()
	if err != nil {
		panic(err)
	}
	return &Server{
		mongoDB: client,
	}
}
