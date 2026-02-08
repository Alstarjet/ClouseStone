package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetData(db *database.MongoClient) http.Handler {
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
			log.Printf("GetData: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		DataDevice, err := devices.ConsultIDs(db, user, deviceid)
		if err != nil {
			log.Printf("GetData: consult IDs error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		DataResponse, err := ConsutDocumentsForDevice(db, DataDevice)
		if err != nil {
			log.Printf("GetData: consult documents error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(DataResponse); err != nil {
			log.Printf("GetData: encode response error: %v", err)
		}
	})
}

func ConsutDocumentsForDevice(db *database.MongoClient, device models.Device) (models.AllData, error) {
	var data models.AllData
	for _, clientID := range device.ClientIDs {
		objID, err := primitive.ObjectIDFromHex(clientID)
		if err != nil {
			return data, err
		}
		filter := bson.D{{Key: "_id", Value: objID}}
		client, _ := db.FindClient(filter)
		data.Clients = append(data.Clients, client)
	}
	for _, chargeID := range device.ChargeIDs {
		objID, err := primitive.ObjectIDFromHex(chargeID)
		if err != nil {
			return data, err
		}
		filter := bson.D{{Key: "_id", Value: objID}}
		charge, _ := db.FindCharge(filter)
		data.Charges = append(data.Charges, charge)
	}
	for _, orderID := range device.OrderIDs {
		objID, err := primitive.ObjectIDFromHex(orderID)
		if err != nil {
			return data, err
		}
		filter := bson.D{{Key: "_id", Value: objID}}
		order, _ := db.FindOrder(filter)
		data.Orders = append(data.Orders, order)
	}
	for _, paymentID := range device.PaymentIDs {
		objID, err := primitive.ObjectIDFromHex(paymentID)
		if err != nil {
			return data, err
		}
		filter := bson.D{{Key: "_id", Value: objID}}
		payment, _ := db.FindPayment(filter)
		data.Payments = append(data.Payments, payment)
	}
	return data, nil
}
