package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/database"
	"log"
	"time"
)

type Server struct {
	mongoDB *database.MongoClient
}

// NewServer crea el servidor, conecta a MongoDB y asegura los índices de sync.
func NewServer() *Server {
	client, err := database.NewMongoClient()
	if err != nil {
		panic(err)
	}

	// Índices del sync v2 (idempotente). No es fatal si falla: el servicio puede
	// arrancar y operar; solo se degrada el rendimiento de la consulta de delta.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.EnsureSyncIndexes(ctx); err != nil {
		log.Printf("NewServer: EnsureSyncIndexes: %v", err)
	}

	return &Server{
		mongoDB: client,
	}
}
