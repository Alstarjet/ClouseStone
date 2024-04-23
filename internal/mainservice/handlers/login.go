package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"financial-Assistant/internal/mainservice/utilities"
	"io"
	"net/http"
	"os"

	"go.mongodb.org/mongo-driver/bson"
)

func Login(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		var data models.DataLogin
		var dataJWT models.DataJWT
		err = json.Unmarshal(body, &data)
		if err != nil {
			return
		}
		if len(data.Device) < 3 {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Error en FrontWithDeviceCode"))
			return
		}
		user, err := db.FindUser(data.Email)
		if err != nil || user.Email == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Error en Usuario o Contraseña"))
			return
		}
		if user.Password == data.Password {
			dataJWT.Email = user.Email
			dataJWT.Name = user.Name
			respJSON, _ := utilities.GenerateToken(dataJWT, os.Getenv("KEY_CODE"))
			newDevice, err := devices.LoginCheckDevice(db, user, data.Device)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !newDevice {
				alldata := ConsutDataForNewDevice(db, user)
				responce := models.JWTresponce{
					Toke:      respJSON,
					Hello:     user.Name,
					NewDevice: true,
					Data:      alldata,
				}
				jsonResponse, err := json.Marshal(responce)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(jsonResponse))
				return
			} else {
				responce := models.JWTresponce{
					Toke:      respJSON,
					Hello:     user.Name,
					NewDevice: false,
				}
				jsonResponse, err := json.Marshal(responce)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(jsonResponse))
				return
			}

		}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("Error en usuario o contraseña"))
	})
}
func ConsutDataForNewDevice(db *database.MongoClient, user models.User) models.AllData {
	filter := bson.D{
		{Key: "usermongoid", Value: user.ID.Hex()},
	}
	payments, _ := db.FindAllPayments(filter)
	charges, _ := db.FindAllCharges(filter)
	clients, _ := db.FindAllClients(filter)
	products, _ := db.FindAllProducts(filter)
	orders, _ := db.FindAllOrders(filter)
	responce := models.AllData{
		Payments: payments,
		Charges:  charges,
		Clients:  clients,
		Products: products,
		Orders:   orders,
	}
	return responce
}
