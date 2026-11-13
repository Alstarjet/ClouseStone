package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func Register(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		var newData models.User
		req, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Register: read body error: %v", err)
			http.Error(w, `{"error":"error al leer la solicitud"}`, http.StatusBadRequest)
			return
		}
		if err = json.Unmarshal(req, &newData); err != nil {
			log.Printf("Register: unmarshal error: %v", err)
			http.Error(w, `{"error":"formato de solicitud inválido"}`, http.StatusBadRequest)
			return
		}

		// Topes de tamaño de cada campo (limits.go): nada de esto llega a la base
		if msg := registerLimits(newData); msg != "" {
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}

		// Validate email format
		if _, err := mail.ParseAddress(newData.Email); err != nil {
			http.Error(w, `{"error":"formato de email inválido"}`, http.StatusBadRequest)
			return
		}

		// Validate password strength
		if len(strings.TrimSpace(newData.Password)) < 8 {
			http.Error(w, `{"error":"la contraseña debe tener al menos 8 caracteres"}`, http.StatusBadRequest)
			return
		}

		// Validate required fields
		if strings.TrimSpace(newData.Name) == "" {
			http.Error(w, `{"error":"el nombre es requerido"}`, http.StatusBadRequest)
			return
		}

		// Que no sea un bot (humancheck.go): la trampa del formulario y, si el servicio
		// tiene la llave de Turnstile, la prueba de Cloudflare. Va después de las
		// validaciones baratas para no gastar la prueba (sirve una vez) en un error de captura
		if err := humanCheck(r.Context(), req, clientIP(r)); err != nil {
			if errors.Is(err, errHumanCheckUnavailable) {
				log.Printf("Register: %v", err)
				http.Error(w, `{"error":"no pudimos revisar tu registro en este momento, inténtalo en un minuto"}`, http.StatusServiceUnavailable)
				return
			}
			http.Error(w, `{"error":"no pudimos confirmar que no eres un robot, inténtalo de nuevo"}`, http.StatusBadRequest)
			return
		}

		// Aceptación de los documentos legales: la fecha la pone el servidor y se
		// guarda con la versión aceptada como constancia (ver terms.go)
		acceptedAt, termsVersion, err := termsAcceptance(req, time.Now())
		if err != nil {
			if errors.Is(err, errTermsVersion) {
				http.Error(w, `{"error":"versión de los términos inválida"}`, http.StatusBadRequest)
				return
			}
			http.Error(w, `{"error":"formato de solicitud inválido"}`, http.StatusBadRequest)
			return
		}
		newData.TermsAcceptedAt = acceptedAt
		newData.TermsVersion = termsVersion

		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		// Check if user already exists
		user, err := db.FindUser(ctx, newData.Email)
		if err == nil && user.Email != "" {
			http.Error(w, `{"error":"email ya en uso, prueba con otro"}`, http.StatusConflict)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newData.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Register: hash password error: %v", err)
			http.Error(w, `{"error":"error al crear el usuario"}`, http.StatusInternalServerError)
			return
		}

		newData.Password = string(hashedPassword)

		result, err := db.RegisterUser(ctx, &newData)
		if err != nil {
			log.Printf("Register: insert user error: %v", err)
			http.Error(w, `{"error":"error al registrar el usuario"}`, http.StatusInternalServerError)
			return
		}

		newData.Password = ""
		respJSON, err := json.Marshal(result)
		if err != nil {
			log.Printf("Register: marshal response error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(respJSON)
	})
}
