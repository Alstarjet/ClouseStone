package handlers

import (
	"financial-Assistant/internal/mainservice/database"
	"log"
	"net/http"
)

func CloseDevice(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		DeviceId := r.Context().Value("deviceid").(string)
		emailRequest := r.Context().Value("Email").(string)
		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = db.RemoveDeviceByUUID(user.ID, DeviceId)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
}
