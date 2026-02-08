package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/charges"
	"financial-Assistant/internal/mainservice/moduls/clients"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"financial-Assistant/internal/mainservice/moduls/orders"
	"financial-Assistant/internal/mainservice/moduls/payments"
	"io"
	"log"
	"net/http"
)

func UploadDataSchedule(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB for data uploads

		deviceID, ok := r.Context().Value(ctxkeys.DeviceID).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		var newData models.RequestUpdate
		req, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("UploadDataSchedule: read body error: %v", err)
			http.Error(w, `{"error":"error reading request"}`, http.StatusBadRequest)
			return
		}
		if err = json.Unmarshal(req, &newData); err != nil {
			log.Printf("UploadDataSchedule: unmarshal error: %v", err)
			http.Error(w, `{"error":"invalid request format"}`, http.StatusBadRequest)
			return
		}

		emailRequest, ok := r.Context().Value(ctxkeys.Email).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Printf("UploadDataSchedule: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		clietsIDs, err := clients.ClientsUploadStone(db, newData.Clients, user)
		if err != nil {
			log.Printf("UploadDataSchedule: clients upload error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		chargesIDs, err := charges.ChargesUploadStone(db, newData.Charges, user)
		if err != nil {
			log.Printf("UploadDataSchedule: charges upload error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		ordersIDs, err := orders.OrdersUploadStone(db, newData.Orders, user)
		if err != nil {
			log.Printf("UploadDataSchedule: orders upload error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		paymentsIDs, err := payments.PaymentsUploadStone(db, newData.Payments, user)
		if err != nil {
			log.Printf("UploadDataSchedule: payments upload error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		err = devices.DevicesUploadStone(db, clietsIDs, chargesIDs, ordersIDs, paymentsIDs, user, deviceID)
		if err != nil {
			log.Printf("UploadDataSchedule: devices upload error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		response := models.BackupResponse{
			Message:    "Datos Respaldados con Éxito",
			Status:     200,
			TypeClient: user.TypeClient,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}
