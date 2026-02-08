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

const maxBodySize = 1 << 20 // 1MB

func Login(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		user, data, err := authenticateUser(db, w, r)
		if err != nil {
			return // response already written by authenticateUser
		}

		Device, err := devices.GetDevice(db, user, data.Device)
		if err != nil {
			log.Printf("Login: get device error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
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
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"Ya existe un dispositivo ligado a esta cuenta, cierra la sesión del dispositivo vinculado para iniciar en uno nuevo"}`))
				return
			}
		}

		issueTokensAndRespond(db, w, user, Device, data.Device, true)
	})
}

func LoginForce(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		user, data, err := authenticateUser(db, w, r)
		if err != nil {
			return
		}

		Device, err := devices.GetDevice(db, user, data.Device)
		if err != nil {
			log.Printf("LoginForce: get device error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		if user.TypeClient == "Quartz" {
			Device.Devices = []models.Device{}
			filter := bson.D{{Key: "_id", Value: user.ID}}
			if err := db.UpdateDevice(filter, Device); err != nil {
				log.Printf("LoginForce: update device error: %v", err)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				return
			}
		}

		issueTokensAndRespond(db, w, user, Device, data.Device, true)
	})
}

// authenticateUser validates credentials and returns the user. Writes error response on failure.
func authenticateUser(db *database.MongoClient, w http.ResponseWriter, r *http.Request) (models.User, models.DataLogin, error) {
	var data models.DataLogin

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("authenticateUser: read body error: %v", err)
		http.Error(w, `{"error":"error reading request"}`, http.StatusBadRequest)
		return models.User{}, data, err
	}

	if err = json.Unmarshal(body, &data); err != nil {
		log.Printf("authenticateUser: unmarshal error: %v", err)
		http.Error(w, `{"error":"invalid request format"}`, http.StatusBadRequest)
		return models.User{}, data, err
	}

	if len(data.Device) < 3 {
		http.Error(w, `{"error":"invalid device identifier"}`, http.StatusBadRequest)
		return models.User{}, data, errInvalidInput
	}

	user, err := db.FindUser(data.Email)
	if err != nil || user.Email == "" {
		http.Error(w, `{"error":"usuario o contraseña incorrectos"}`, http.StatusUnauthorized)
		return models.User{}, data, errInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(data.Password)); err != nil {
		http.Error(w, `{"error":"usuario o contraseña incorrectos"}`, http.StatusUnauthorized)
		return models.User{}, data, err
	}

	return user, data, nil
}

// issueTokensAndRespond generates tokens, stores them, sets cookie, and writes JSON response.
func issueTokensAndRespond(db *database.MongoClient, w http.ResponseWriter, user models.User, device models.UserDevices, deviceUUID string, includeData bool) {
	accessToken, expiresJWT, err := utilities.GenerateToken(user, deviceUUID, os.Getenv("ACCESS_SECRET"))
	if err != nil {
		log.Printf("issueTokens: generate access token error: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	refreshToken, expires, err := utilities.GenerateRefreshToken(user, deviceUUID, os.Getenv("REFRESH_SECRET"))
	if err != nil {
		log.Printf("issueTokens: generate refresh token error: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	if err = devices.AddDeviceAndRefreshToken(db, device, refreshToken, expires, deviceUUID); err != nil {
		log.Printf("issueTokens: store device/token error: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	response := models.JWTresponce{
		Toke:       accessToken,
		Expires:    expiresJWT,
		UserName:   user.Name + " " + user.LastName,
		TypeClient: user.TypeClient,
	}

	if includeData {
		response.Data = consultDataForNewDevice(db, user)
	}

	SetRefreshCookie(w, refreshToken, expires)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("issueTokens: encode response error: %v", err)
	}
}

func consultDataForNewDevice(db *database.MongoClient, user models.User) models.AllData {
	filter := bson.D{
		{Key: "usermongoid", Value: user.ID.Hex()},
	}
	payments, _ := db.FindAllPayments(filter)
	charges, _ := db.FindAllCharges(filter)
	clients, _ := db.FindAllClients(filter)
	orders, _ := db.FindAllOrders(filter)
	return models.AllData{
		Payments: payments,
		Charges:  charges,
		Clients:  clients,
		Orders:   orders,
	}
}

var (
	errInvalidInput       = errSentinel("invalid input")
	errInvalidCredentials = errSentinel("invalid credentials")
)

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
