package main

import (
	"fmt"
	"log"
	"net/http"

	"financial-Assistant/internal/mainservice"
	"financial-Assistant/internal/mainservice/database"
)

func main() {
	client, err := database.NewMongoClient()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			panic(err)
		}
	}()

	user := &database.User{
		Name: "John Doe Star Max",
		Age:  30,
	}

	if err := client.InsertUser(user); err != nil {
		panic(err)
	}

	fmt.Println("Successfully inserted user!")
	// Crea un nuevo enrutador Gorilla Mux.
	router := mainservice.NewRouter()

	// Inicia el servidor HTTP.
	log.Println("Iniciando el servidor en http://localhost:8080...")
	error := http.ListenAndServe(":8080", router)
	if error != nil {
		log.Fatal("No se pudo iniciar el servidor: ", err)
	}
}
