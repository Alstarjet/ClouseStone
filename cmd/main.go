package main

import (
	"log"
	"net/http"

	"financial-Assistant/internal/mainservice"
)

func main() {

	// Crea un nuevo enrutador Gorilla Mux.
	server := mainservice.NewServer()
	router := mainservice.NewRouter(server)

	// Inicia el servidor HTTP.
	log.Println("Iniciando el servidor en http://localhost:8080...")
	error := http.ListenAndServe(":8080", router)
	if error != nil {
		log.Fatal("No se pudo iniciar el servidor: ", error)
	}
}
