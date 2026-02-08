package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func RefreshToken(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(ctxkeys.UserID).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		deviceID, ok := r.Context().Value(ctxkeys.DeviceID).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		cookieToken, ok := r.Context().Value(ctxkeys.Cookie).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		user, err := db.FindUserByID(userID)
		if err != nil {
			log.Printf("RefreshToken: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		filter := bson.D{{Key: "_id", Value: user.ID}}
		deviceDoc, err := db.FindDevice(filter)
		if err != nil {
			log.Printf("RefreshToken: find device error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		// Find the matching device and validate refresh token hash
		hashedToken := utilities.HashToken(cookieToken)
		deviceFound := false

		for i := 0; i < len(deviceDoc.Devices); i++ {
			if deviceID != deviceDoc.Devices[i].UUID {
				continue
			}
			deviceFound = true

			if deviceDoc.Devices[i].Refreshtoken.Token != hashedToken {
				log.Printf("RefreshToken: token hash mismatch for device %s", deviceID)
				http.Error(w, `{"error":"invalid refresh token"}`, http.StatusUnauthorized)
				return
			}

			// Check if refresh token needs rotation
			now := time.Now()
			environment := os.Getenv("ENVIRONMENT")
			var rotationWindow time.Duration
			if environment == "local" {
				rotationWindow = 1 * time.Minute
			} else {
				rotationWindow = 10 * 24 * time.Hour
			}

			if deviceDoc.Devices[i].Refreshtoken.DateEnd.Before(now.Add(rotationWindow)) {
				newRefreshToken, expires, err := utilities.GenerateRefreshToken(user, deviceID, os.Getenv("REFRESH_SECRET"))
				if err != nil {
					log.Printf("RefreshToken: generate refresh token error: %v", err)
					http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
					return
				}
				newHash := utilities.HashToken(newRefreshToken)
				if err = db.UpdateDeviceRefreshToken(deviceDoc.ID, deviceID, newHash, expires); err != nil {
					log.Printf("RefreshToken: update refresh token error: %v", err)
					http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
					return
				}
				SetRefreshCookie(w, newRefreshToken, expires)
			}
			break
		}

		if !deviceFound {
			http.Error(w, `{"error":"device not found"}`, http.StatusUnauthorized)
			return
		}

		accessToken, expiresJWT, err := utilities.GenerateToken(user, deviceID, os.Getenv("ACCESS_SECRET"))
		if err != nil {
			log.Printf("RefreshToken: generate access token error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		response := models.JWTresponce{
			Toke:       accessToken,
			Expires:    expiresJWT,
			UserName:   user.Name,
			TypeClient: user.TypeClient,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("RefreshToken: encode response error: %v", err)
		}
	})
}
