package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/utilities"
	"net/http"
	"os"
)

func RefreshMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the refresh token from the cookies
		cookie, err := r.Cookie("refresh_token")
		if err != nil {
			if err == http.ErrNoCookie {
				w.WriteHeader(http.StatusUnauthorized)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("Missing refresh token cookie"))
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Error retrieving refresh token cookie"))
			return
		}

		// Decode the token
		info, _ := utilities.DecodeToken(cookie.Value, os.Getenv("KEY_CODE"))
		if info == nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Invalid refresh token"))
			return
		}

		// Add email to the request context
		ctx := context.WithValue(r.Context(), "UserId", info["userid"].(string))
		ctx = context.WithValue(ctx, "DeviceId", info["deviceid"].(string))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
