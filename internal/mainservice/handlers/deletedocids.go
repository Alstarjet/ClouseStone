package handlers

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"log"
	"net/http"
)

func DeleteDocIds(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		deviceid := queryParams.Get("deviceid")
		emailRequest := r.Context().Value("Email").(string)
		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		err = devices.DeleteIDsForDevice(db, user, deviceid)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
}
