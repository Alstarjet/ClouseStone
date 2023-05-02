package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/handlers"
	"financial-Assistant/internal/mainservice/utilities"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// NewRouter crea un nuevo router Gorilla Mux y configura sus rutas.
func NewRouter(server *Server) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.Handle("/hello", AuthMiddleware(handlers.Hello(server.mongoDB))).Methods(http.MethodPost)

	router.Handle("/register", handlers.Register(server.mongoDB)).Methods(http.MethodPost)
	router.Handle("/login", handlers.Login(server.mongoDB)).Methods(http.MethodPost)
	return router
}
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Missing Authorization header"))
			return
		}

		// Check if Authorization header has Bearer token
		auth := strings.Split(authHeader, " ")
		if len(auth) != 2 || auth[0] != "Bearer" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Invalid Authorization header format"))
			return
		}

		info, _ := utilities.DecodeToken(auth[1], "TTE68sTTuasd")
		if info == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Tu token no sirve"))
			return
		}
		ctx := context.WithValue(r.Context(), "Email", info["email"].(string))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
