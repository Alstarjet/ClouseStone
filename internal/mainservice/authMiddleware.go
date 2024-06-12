package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/utilities"
	"net/http"
	"os"
	"strings"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Missing Authorization header"))
			return
		}

		// Check if Authorization header has Bearer token
		auth := strings.Split(authHeader, " ")
		if len(auth) != 2 || auth[0] != "Bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Invalid Authorization header format"))
			return
		}

		info, _ := utilities.DecodeToken(auth[1], os.Getenv("KEY_CODE"))
		if info == nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Tu token no sirve"))
			return
		}
		ctx := context.WithValue(r.Context(), "Email", info["email"].(string))
		ctx = context.WithValue(ctx, "DeviceId", info["deviceid"].(string))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
