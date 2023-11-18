package mainservice

import (
	"financial-Assistant/internal/mainservice/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

// NewRouter crea un nuevo router Gorilla Mux y configura sus rutas.
func NewRouter(server *Server) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.Handle("/hello", AuthMiddleware(handlers.Hello(server.mongoDB))).Methods(http.MethodPost)
	router.Handle("/UploadData", AuthMiddleware(handlers.UploadDataSchedule(server.mongoDB))).Methods(http.MethodPost)

	router.Handle("/register", handlers.Register(server.mongoDB)).Methods(http.MethodPost)
	router.Handle("/login", handlers.Login(server.mongoDB)).Methods(http.MethodPost)
	router.Handle("/addProduct", handlers.Products(server.mongoDB)).Methods(http.MethodPost)
	router.Handle("/allProduct", handlers.FindAllProducts(server.mongoDB)).Methods(http.MethodGet)
	return router
}
