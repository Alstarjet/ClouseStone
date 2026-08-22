package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// GetDataSince devuelve todos los documentos del usuario cuyo backupat (hora en
// que el SERVIDOR los recibió) sea >= al parámetro `since` (RFC3339). Sin
// `since` devuelve el dataset completo (primera descarga). La respuesta incluye
// servertime para que el cliente guarde su cursor con reloj del servidor, no
// del dispositivo.
//
// A diferencia de /GetData, no depende de las colas por dispositivo: es un
// delta puro por fecha de llegada, idéntico para cualquier dispositivo.
func GetDataSince(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailRequest, ok := r.Context().Value(ctxkeys.Email).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Printf("GetDataSince: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		// La hora del servidor se toma ANTES de consultar: si algo llega
		// mientras respondemos, el próximo delta lo vuelve a incluir (el
		// upsert por uuid en el cliente hace inofensivo el traslape).
		serverTime := time.Now().UTC()

		filter := bson.D{{Key: "usermongoid", Value: user.ID.Hex()}}
		if sinceParam := r.URL.Query().Get("since"); sinceParam != "" {
			since, err := time.Parse(time.RFC3339, sinceParam)
			if err != nil {
				http.Error(w, `{"error":"formato de since inválido, se espera RFC3339"}`, http.StatusBadRequest)
				return
			}
			filter = append(filter, bson.E{Key: "backupat", Value: bson.D{{Key: "$gte", Value: since}}})
		}

		response := models.AllDataSince{ServerTime: serverTime}
		if response.Clients, err = db.FindAllClients(filter); err != nil {
			log.Printf("GetDataSince: clients error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		if response.Charges, err = db.FindAllCharges(filter); err != nil {
			log.Printf("GetDataSince: charges error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		if response.Orders, err = db.FindAllOrders(filter); err != nil {
			log.Printf("GetDataSince: orders error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		if response.Payments, err = db.FindAllPayments(filter); err != nil {
			log.Printf("GetDataSince: payments error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("GetDataSince: encode response error: %v", err)
		}
	})
}
