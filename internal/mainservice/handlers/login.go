package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"financial-Assistant/internal/mainservice/utilities"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// maxBodySize es el tope del cuerpo de /register y /login: son unos cuantos campos
// cortos (limits.go). Era de 1 MB (S-02).
const maxBodySize = 16 << 10 // 16 KB

// Login abre la sesión en el dispositivo que manda el cliente. No hay límite de dispositivos en ningún plan: cada uno
// queda registrado con su propio refresh token y sólo se cierra con /CloseDevice desde ese mismo dispositivo.
//
// El contexto trae el tiempo límite propio de la ruta (route.go): la respuesta lleva todos los datos de la cuenta.
func Login(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		ctx := r.Context()

		user, data, err := authenticateUser(ctx, db, w, r)
		if err != nil {
			return // response already written by authenticateUser
		}

		issueTokensAndRespond(ctx, db, w, user, data.Device, true)
	})
}

// LoginForce era la entrada del plan de un solo dispositivo (Quartz): desvinculaba los demás dispositivos.
//
// Deprecated: ya no hay límite de dispositivos. Se conserva para las versiones de la app que aún lo llaman y hace
// exactamente lo mismo que Login (no cierra las otras sesiones).
func LoginForce(db *database.MongoClient) http.Handler {
	return Login(db)
}

// userFinder busca la cuenta por correo. Lo cumple *database.MongoClient; las
// pruebas usan uno en memoria.
type userFinder interface {
	FindUser(ctx context.Context, email string) (models.User, error)
}

// authenticateUser validates credentials and returns the user. Writes error response on failure.
func authenticateUser(ctx context.Context, db userFinder, w http.ResponseWriter, r *http.Request) (models.User, models.DataLogin, error) {
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

	if len(data.Device) < 3 || len(data.Device) > maxDeviceLength {
		http.Error(w, `{"error":"invalid device identifier"}`, http.StatusBadRequest)
		return models.User{}, data, errInvalidInput
	}

	// Freno de intentos (loginlimit.go): pasado el tope no se toca la base ni bcrypt
	ip := rateLimitIP(r)
	if wait := loginLimits.blocked(ip, data.Email); wait > 0 {
		tooManyAttempts(w, wait)
		return models.User{}, data, errTooManyAttempts
	}
	// Un correo o una contraseña que no caben en una cuenta real (limits.go) no se
	// buscan; el fallo cuenta sólo para la IP, para no guardar el correo gigante
	if len(data.Email) > maxEmailLength || len(data.Password) > maxPasswordBytes {
		loginLimits.failed(ip, "")
		http.Error(w, `{"error":"usuario o contraseña incorrectos"}`, http.StatusUnauthorized)
		return models.User{}, data, errInvalidCredentials
	}

	user, err := db.FindUser(ctx, data.Email)
	// Si la base falla no es una contraseña incorrecta: 500, para que la persona
	// reintente en vez de creer que se equivocó (I-09)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("authenticateUser: find user error: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return models.User{}, data, err
	}
	if err != nil || user.Email == "" {
		// Que tarde lo mismo que con un correo que sí existe
		burnPasswordCheck(data.Password)
		loginLimits.failed(ip, data.Email)
		http.Error(w, `{"error":"usuario o contraseña incorrectos"}`, http.StatusUnauthorized)
		return models.User{}, data, errInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(data.Password)); err != nil {
		loginLimits.failed(ip, data.Email)
		http.Error(w, `{"error":"usuario o contraseña incorrectos"}`, http.StatusUnauthorized)
		return models.User{}, data, err
	}

	loginLimits.succeeded(data.Email)
	return user, data, nil
}

// issueTokensAndRespond generates tokens, stores them, sets cookie, and writes JSON response.
func issueTokensAndRespond(ctx context.Context, db *database.MongoClient, w http.ResponseWriter, user models.User, deviceUUID string, includeData bool) {
	// Primero los datos: si no se pudieron leer completos se responde 500 sin
	// registrar nada, en vez de un 200 con la cuenta vacía o incompleta (I-09)
	var data models.AllData
	if includeData {
		var err error
		if data, err = consultDataForNewDevice(ctx, db, user); err != nil {
			log.Printf("issueTokens: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
	}

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

	if err = devices.AddDeviceAndRefreshToken(ctx, db, user, refreshToken, expires, deviceUUID); err != nil {
		log.Printf("issueTokens: store device/token error: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	response := withPlan(models.JWTresponce{
		Toke:         accessToken,
		Expires:      expiresJWT,
		UserName:     user.Name + " " + user.LastName,
		RefreshToken: refreshToken,
	}, user)
	response.Data = data

	SetRefreshCookie(w, refreshToken, expires)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("issueTokens: encode response error: %v", err)
	}
}

// accountReader es lo que el login lee de la base para mandar los datos de la
// cuenta. Lo cumple *database.MongoClient; las pruebas usan uno en memoria.
type accountReader interface {
	FindAllClients(ctx context.Context, filter interface{}) ([]models.Client, error)
	FindAllCharges(ctx context.Context, filter interface{}) ([]models.Charge, error)
	FindAllOrders(ctx context.Context, filter interface{}) ([]models.Charge, error)
	FindAllPayments(ctx context.Context, filter interface{}) ([]models.Payment, error)
}

// consultDataForNewDevice lee todos los datos de la cuenta para el login. Si
// cualquiera de las cuatro consultas falla regresa el error: nunca una parte.
func consultDataForNewDevice(ctx context.Context, db accountReader, user models.User) (models.AllData, error) {
	filter := bson.D{
		{Key: "usermongoid", Value: user.ID.Hex()},
	}
	var data models.AllData
	var err error
	if data.Payments, err = db.FindAllPayments(ctx, filter); err != nil {
		return models.AllData{}, fmt.Errorf("consultDataForNewDevice: pagos: %w", err)
	}
	if data.Charges, err = db.FindAllCharges(ctx, filter); err != nil {
		return models.AllData{}, fmt.Errorf("consultDataForNewDevice: cargos: %w", err)
	}
	if data.Clients, err = db.FindAllClients(ctx, filter); err != nil {
		return models.AllData{}, fmt.Errorf("consultDataForNewDevice: clientes: %w", err)
	}
	if data.Orders, err = db.FindAllOrders(ctx, filter); err != nil {
		return models.AllData{}, fmt.Errorf("consultDataForNewDevice: pedidos: %w", err)
	}
	return data, nil
}

var (
	errInvalidInput       = errSentinel("invalid input")
	errInvalidCredentials = errSentinel("invalid credentials")
	errTooManyAttempts    = errSentinel("too many attempts")
)

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
