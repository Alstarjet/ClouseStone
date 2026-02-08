package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/utilities"
	"log"
	"net/http"
	"os"
)

func RefreshMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenValue string

		// 1. Check header first (native app)
		if headerToken := r.Header.Get("X-Refresh-Token"); headerToken != "" {
			tokenValue = headerToken
		} else {
			// 2. Fallback to cookie (PWA/browser)
			cookie, err := r.Cookie("refresh_token")
			if err != nil {
				if err == http.ErrNoCookie {
					http.Error(w, `{"error":"missing refresh token"}`, http.StatusUnauthorized)
					return
				}
				log.Printf("RefreshMiddleware: cookie error: %v", err)
				http.Error(w, `{"error":"error reading refresh token"}`, http.StatusBadRequest)
				return
			}
			tokenValue = cookie.Value
		}

		info, err := utilities.DecodeToken(tokenValue, os.Getenv("REFRESH_SECRET"), "refresh")
		if err != nil {
			log.Printf("RefreshMiddleware: token decode error: %v", err)
			http.Error(w, `{"error":"invalid or expired refresh token"}`, http.StatusUnauthorized)
			return
		}

		userID, ok := info["userid"].(string)
		if !ok {
			http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
			return
		}
		deviceID, ok := info["deviceid"].(string)
		if !ok {
			http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxkeys.UserID, userID)
		ctx = context.WithValue(ctx, ctxkeys.DeviceID, deviceID)
		ctx = context.WithValue(ctx, ctxkeys.Cookie, tokenValue)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
