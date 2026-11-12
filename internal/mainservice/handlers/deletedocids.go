package handlers

import (
	"context"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"log"
	"net/http"
)

// DeleteDocIds vacía la cola de pendientes (legado) del dispositivo. La app de
// producción todavía lo llama después de /GetData desde una pantalla vieja; ya
// es inofensivo: sólo vacía las listas de ese dispositivo, sin reescribir el
// documento de dispositivos (I-07). Se quita cuando se despliegue el frontend sin
// esa pantalla (I-14).
func DeleteDocIds(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		deviceid := queryParams.Get("deviceid")

		emailRequest, ok := r.Context().Value(ctxkeys.Email).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		user, err := db.FindUser(ctx, emailRequest)
		if err != nil {
			log.Printf("DeleteDocIds: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		if err = devices.DeleteIDsForDevice(ctx, db, user, deviceid); err != nil {
			log.Printf("DeleteDocIds: delete IDs error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
}
