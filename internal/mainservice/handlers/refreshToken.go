package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func RefreshToken(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		UserId := r.Context().Value("UserId").(string)
		DeviceId := r.Context().Value("DeviceId").(string)
		Cookie := r.Context().Value("Cookie").(string)

		user, err := db.FindUserByID(UserId)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		filter := bson.D{
			{Key: "_id", Value: user.ID},
		}
		DeviceDoc, err := db.FindDevice(filter)
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		for i := 0; i < len(DeviceDoc.Devices); i++ {
			if DeviceId == DeviceDoc.Devices[i].UUID {
				if DeviceDoc.Devices[i].Refreshtoken.Token != Cookie {
					log.Printf("Error: %v\n", err)
					http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusInternalServerError)
					return
				}

				now := time.Now()
				fmt.Println(now)
				environment := os.Getenv("ENVIRONMENT")
				var diferTime time.Duration
				if environment == "local" {
					diferTime = 1 * time.Minute
				} else {
					diferTime = 10 * 24 * time.Hour
				}
				fmt.Println(DeviceDoc.Devices[i].Refreshtoken.DateEnd)
				if DeviceDoc.Devices[i].Refreshtoken.DateEnd.Before(now.Add(diferTime)) {
					RefreshToken, expires, err := utilities.GenerateRefreshToken(user, DeviceId, os.Getenv("KEY_CODE"))
					if err != nil {
						log.Printf("Error: %v\n", err)
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
					err = db.UpdateDeviceRefreshToken(DeviceDoc.ID, DeviceId, RefreshToken, expires)
					if err != nil {
						log.Printf("Error: %v\n", err)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					http.SetCookie(w, &http.Cookie{
						Name:     "refresh_token",
						Value:    RefreshToken,
						HttpOnly: true,
						Path:     "/",
						Expires:  expires,
					})
					fmt.Println("Es hora de un nuevop refresh")
				} else {
					fmt.Println("No toca refresh")
				}
				break
			}
		}
		JSONToken, ExpiresJWT, err := utilities.GenerateToken(user, DeviceId, os.Getenv("KEY_CODE"))
		if err != nil {
			log.Printf("Error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		responce := models.JWTresponce{
			Toke:       JSONToken,
			Expires:    ExpiresJWT,
			UserName:   user.Name,
			TypeClient: user.TypeClient,
		}

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
