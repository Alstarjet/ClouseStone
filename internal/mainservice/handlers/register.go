package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"io"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func Register(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.User
		req, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, "Error al registrar el usuario", http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(req, &newData)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, "Error al registrar el usuario", http.StatusInternalServerError)
			return
		}
		// Check if user already exists
		user, err := db.FindUser(newData.Email)
		if err == nil && user.Email != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Email ya en uso, prueba con otro"))
			return
		}

		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newData.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, "Error al crear el usuario", http.StatusInternalServerError)
			return
		}

		// Set the hashed password
		newData.Password = string(hashedPassword)

		// Register the user
		reques, err := db.RegisterUser(&newData)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, "Error al registrar el usuario", http.StatusInternalServerError)
			return
		}

		// Respond with the new user details (without password)
		newData.Password = ""
		respJSON, err := json.Marshal(reques)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
	})
}
