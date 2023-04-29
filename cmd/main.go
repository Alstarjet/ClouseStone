package main

import (
	"log"
	"net/http"

	"financial-Assistant/internal/mainservice"
)

func main() {
	// Crea un nuevo enrutador Gorilla Mux.
	router := mainservice.NewRouter()

	// Inicia el servidor HTTP.
	log.Println("Iniciando el servidor en http://localhost:8080...")
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatal("No se pudo iniciar el servidor: ", err)
	}
}
