package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"

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

		// Check if user already exists
		user, err := db.FindUser(newData.Email)
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

		result, err := db.RegisterUser(&newData)
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
