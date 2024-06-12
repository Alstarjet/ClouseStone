package handlers

import (
	"encoding/json"
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
		emailRequest := r.Context().Value("Email").(string)
		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		DataDevice, err := devices.ConsultIDs(db, user, deviceid)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		DataResponse, err := ConsutDocumentsForDevice(db, DataDevice)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse, err := json.Marshal(DataResponse)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonResponse))
	})
}
func ConsutDocumentsForDevice(db *database.MongoClient, device models.Device) (models.AllData, error) {
	var data models.AllData
	for _, clientID := range device.ClientIDs {
		objID, err := primitive.ObjectIDFromHex(clientID)
		if err != nil {
			return data, err
		}
		filter := bson.D{
			{Key: "_id", Value: objID},
		}
		client, _ := db.FindClient(filter)
		data.Clients = append(data.Clients, client)
	}
	for _, chargeID := range device.ChargeIDs {
		objID, err := primitive.ObjectIDFromHex(chargeID)
		if err != nil {
			return data, err
		}
		filter := bson.D{
			{Key: "_id", Value: objID},
		}
		charge, _ := db.FindCharge(filter)
		data.Charges = append(data.Charges, charge)
	}
	for _, orderID := range device.OrderIDs {
		objID, err := primitive.ObjectIDFromHex(orderID)
		if err != nil {
			return data, err
		}
		filter := bson.D{
			{Key: "_id", Value: objID},
		}
		order, _ := db.FindOrder(filter)
		data.Orders = append(data.Orders, order)
	}

	for _, paymentID := range device.PaymentIDs {
		objID, err := primitive.ObjectIDFromHex(paymentID)
		if err != nil {
			return data, err
		}
		filter := bson.D{
			{Key: "_id", Value: objID},
		}
		payment, _ := db.FindPayment(filter)
		data.Payments = append(data.Payments, payment)
	}
	return data, nil
}
