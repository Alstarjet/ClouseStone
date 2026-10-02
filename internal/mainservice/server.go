package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/moduls/catalog"
	"log"
	"os"
	"strconv"
	"time"
)

type Server struct {
	mongoDB *database.MongoClient
	// catalog sirve el catálogo curado de productos y kits (solo lectura).
	catalog *catalog.Service
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

	// Índices del catálogo curado (idempotente, best-effort): la unicidad de claves y slugs
	// se pide a Mongo al arrancar/desplegar.
	if err := client.EnsureCatalogIndexes(ctx); err != nil {
		log.Printf("NewServer: EnsureCatalogIndexes: %v", err)
	}

	return &Server{
		mongoDB: client,
		catalog: catalog.NewService(client, catalogTTL()),
	}
}

// catalogTTL lee CATALOG_CACHE_TTL_SECONDS (segundos que el catálogo se mantiene en
// memoria antes de recargarse de Mongo). Por defecto, 5 minutos.
func catalogTTL() time.Duration {
	if v, err := strconv.Atoi(os.Getenv("CATALOG_CACHE_TTL_SECONDS")); err == nil && v > 0 {
		return time.Duration(v) * time.Second
	}
	return catalog.DefaultTTL
}
