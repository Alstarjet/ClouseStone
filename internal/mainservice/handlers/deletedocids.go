package handlers

import (
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"log"
	"net/http"
)

func DeleteDocIds(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		deviceid := queryParams.Get("deviceid")

		emailRequest, ok := r.Context().Value(ctxkeys.Email).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Printf("DeleteDocIds: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		if err = devices.DeleteIDsForDevice(db, user, deviceid); err != nil {
			log.Printf("DeleteDocIds: delete IDs error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
}
