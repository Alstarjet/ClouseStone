package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"io"
	"net/http"
)

func Login(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		var data models.DataLogin
		var dataJWT models.DataJWT
		err = json.Unmarshal(body, &data)
		if err != nil {
			return
		}
		user, err := db.FindUser(data.Email)
		if err != nil || user.Email == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Error en Usuario o Contraseña"))
			return
		}
		if user.Password == data.Password {
			dataJWT.Email = user.Email
			dataJWT.Name = user.Name
			respJSON, _ := utilities.GenerateToken(dataJWT, "TTE68sTTuasd")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(respJSON))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("Error en usuario o contraseña"))
	})
}
