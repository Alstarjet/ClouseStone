package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/utilities"
	"log"
	"net/http"
	"os"
	"strings"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		auth := strings.Split(authHeader, " ")
		if len(auth) != 2 || auth[0] != "Bearer" {
			http.Error(w, `{"error":"invalid Authorization header format"}`, http.StatusUnauthorized)
			return
		}

		info, err := utilities.DecodeToken(auth[1], os.Getenv("ACCESS_SECRET"), "access")
		if err != nil {
			log.Printf("AuthMiddleware: token decode error: %v", err)
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		email, ok := info["email"].(string)
		if !ok {
			http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
			return
		}
		deviceID, ok := info["deviceid"].(string)
		if !ok {
			http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxkeys.Email, email)
		ctx = context.WithValue(ctx, ctxkeys.DeviceID, deviceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
