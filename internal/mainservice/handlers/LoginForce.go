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

	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

func LoginForce(db *database.MongoClient) http.Handler {
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
			Device.Devices = []models.Device{}
			filter := bson.D{{Key: "_id", Value: user.ID}}
			db.UpdateDevice(filter, Device)
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
