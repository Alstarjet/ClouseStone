package main

import (
	"log"
	"net/http"
	"os"

	"financial-Assistant/internal/mainservice"

	"github.com/rs/cors"
)

func main() {

	// Crea un nuevo enrutador Gorilla Mux.
	server := mainservice.NewServer()
	router := mainservice.NewRouter(server)
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{os.Getenv("URL_FRONT")},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true, // Asegúrate de permitir credenciales
	})
	handler := c.Handler(router)

	// Define una función de manejo que agrega la CSP al encabezado de respuesta.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src https://example.com")
		handler.ServeHTTP(w, r)
	})

	// Inicia el servidor HTTP.
	log.Println("Iniciando el servidor en http://localhost:8080...")
	error := http.ListenAndServe(":8080", nil)
	if error != nil {
		log.Fatal("No se pudo iniciar el servidor: ", error)
	}
}
