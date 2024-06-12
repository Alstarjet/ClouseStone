package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"financial-Assistant/internal/mainservice/utilities"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

func Login(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}
		var data models.DataLogin
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Printf("Error: %v\n", err)
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
			log.Printf("Error: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Usuario o contraseña incorrectos"))
			return
		}
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(data.Password))
		if err != nil {
			log.Printf("Error: %v\n", err)
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Usuario o contraseña incorrectos"))
			return
		}
		Device, err := devices.GetDevice(db, user, data.Device)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if user.TypeClient == "Quartz" {
			var newD = true
			for _, device := range Device.Devices {
				if device.UUID == data.Device {
					newD = false
				}
			}
			if newD && len(Device.Devices) > 0 {
				w.WriteHeader(http.StatusForbidden)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("Ya existe un dispositivo ligado a esta cuenta, salir del dispositivo vingulado para iniciar en uno nuevo"))
				return
			}
		}
		Alldata := ConsutDataForNewDevice(db, user)
		JSONToken, expiresJWT, err := utilities.GenerateToken(user, data.Device, os.Getenv("KEY_CODE"))
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		RefreshToken, expires, err := utilities.GenerateRefreshToken(user, data.Device, os.Getenv("KEY_CODE"))
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = devices.AddDeviceAndRefreshToken(db, Device, RefreshToken, expires, data.Device)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responce := models.JWTresponce{
			Toke:       JSONToken,
			Expires:    expiresJWT,
			UserName:   user.Name,
			Data:       Alldata,
			TypeClient: user.TypeClient,
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    RefreshToken,
			HttpOnly: true,
			Path:     "/",
			Expires:  expires,
		})

		jsonResponse, err := json.Marshal(responce)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonResponse))

	})
}
func ConsutDataForNewDevice(db *database.MongoClient, user models.User) models.AllData {
	filter := bson.D{
		{Key: "usermongoid", Value: user.ID.Hex()},
	}
	payments, _ := db.FindAllPayments(filter)
	charges, _ := db.FindAllCharges(filter)
	clients, _ := db.FindAllClients(filter)
	orders, _ := db.FindAllOrders(filter)
	responce := models.AllData{
		Payments: payments,
		Charges:  charges,
		Clients:  clients,
		Orders:   orders,
	}
	return responce
}
func AddDeviceAndRefreshToken(Device *models.UserDevices, RefreshToken string, UUID string) time.Time {
	environment := os.Getenv("ENVIRONMENT")
	var dateEnd time.Time
	if environment == "local" {
		dateEnd = time.Now().Add(2 * time.Minute)
	} else {
		dateEnd = time.Now().Add(22 * 24 * time.Hour)
	}
	newDevice := models.Device{
		UUID: UUID,
		Refreshtoken: models.Refreshtoken{
			Token:   RefreshToken,
			DateEnd: dateEnd, // Agrega 22 días a la fecha actual
		},
	}
	found := false
	for i, device := range Device.Devices {
		if device.UUID == newDevice.UUID {
			found = true
			Device.Devices[i].Refreshtoken.Token = newDevice.Refreshtoken.Token
			Device.Devices[i].Refreshtoken.DateEnd = newDevice.Refreshtoken.DateEnd
		}
	}
	if !found {
		Device.Devices = append(Device.Devices, newDevice)
	}
	return dateEnd
}
