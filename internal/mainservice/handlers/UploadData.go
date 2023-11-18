package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/charges"
	"financial-Assistant/internal/mainservice/moduls/clients"
	"financial-Assistant/internal/mainservice/moduls/payments"
	"io"
	"log"
	"net/http"
)

func UploadDataSchedule(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.RequestUpload
		req, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(req, &newData)
		if err != nil {
			log.Println(err)
			response, _ := json.Marshal(err)
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write(response)
			return
		}
		emailRequest := r.Context().Value("Email").(string)
		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = clients.ClientsUpload(db, newData.Clients, user)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = payments.PaymentsUpdateStone(db, newData.Payments, user)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = charges.ChargesUpdateStone(db, newData.Charges, user)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		response := models.BackupResponse{
			Message: "Datos Respaldados con Éxito",
			Status:  200,
		}
		jsonResponse, _ := json.Marshal(response)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse)

	})
}
