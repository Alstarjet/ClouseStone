package handlers

import (
	"net/http"
	"os"
	"time"
)

func SetRefreshCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/GetJwt",
		Domain:   os.Getenv("COOKIE_DOMAIN"),
		Expires:  expires,
	})
}
