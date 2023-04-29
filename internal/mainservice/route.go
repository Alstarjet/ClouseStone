package mainservice

import (
	"financial-Assistant/internal/mainservice/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

// NewRouter crea un nuevo router Gorilla Mux y configura sus rutas.
func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.Handle("/hello", handlers.Hello()).Methods(http.MethodGet)

	return router
}
