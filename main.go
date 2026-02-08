package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"financial-Assistant/internal/mainservice"

	"github.com/rs/cors"
)

func main() {
	server := mainservice.NewServer()
	router := mainservice.NewRouter(server)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{os.Getenv("URL_FRONT")},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Refresh-Token"},
		AllowCredentials: true,
	})

	handler := c.Handler(router)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Println("Iniciando el servidor en :8080...")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error en el servidor: %v", err)
		}
	}()

	// Wait for SIGTERM (Cloud Run sends this before shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("Apagando servidor...")

	// Give in-flight requests time to finish
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown forzado: %v", err)
	}

	log.Println("Servidor apagado correctamente")
}
