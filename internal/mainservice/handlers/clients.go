package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/payments"
	"io"
	"log"
	"net/http"
)

func AddClient(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.ClientRegister
		req, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		_ = json.Unmarshal(req, &newData)
		if newData.Address == "" || newData.ClientUuid == "" || newData.DayWork == "" || newData.Name == "" || newData.Phone == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("No complete info"))
			return
		}
		emailRequest := r.Context().Value("Email").(string)
		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		bollo := payments.CreateNewCliente(db, newData, user)
		if bollo {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Se registro"))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("No se registro"))
		}

	})
}
