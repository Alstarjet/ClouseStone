package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"io"
	"log"
	"net/http"
)

func Hello(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		updateReq := r.Context().Value("Email").(string)
		fmt.Println("estamos en hello", updateReq)
		user, err := db.FindUser(updateReq)
		fmt.Println("Esto salio de mongo", user)
		if err != nil || user.Email == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Error en Usuario o Contraseña"))
			return
		}
		w.Write([]byte("Hello, tu token es correcto"))
	})
}
func Register(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.User
		req, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(req, &newData)
		user, err := db.FindUser(newData.Email)
		if err == nil && user.Email != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Email ya en Uso, prueba con otro"))
			return
		}
		reques, err := db.RegisterUser(&newData)
		if err != nil {
			log.Println(err)
		}
		respJSON, err := json.Marshal(reques)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
	})
}
