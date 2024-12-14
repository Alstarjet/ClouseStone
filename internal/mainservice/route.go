package mainservice

import (
	"financial-Assistant/internal/mainservice/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

// NewRouter crea un nuevo router Gorilla Mux y configura sus rutas. //nuevo comentario
func NewRouter(server *Server) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.Handle("/UploadData", AuthMiddleware(handlers.UploadDataSchedule(server.mongoDB))).Methods(http.MethodPost)
	router.Handle("/GetData", AuthMiddleware(handlers.GetData(server.mongoDB))).Methods(http.MethodGet)
	router.Handle("/DeleteIds", AuthMiddleware(handlers.DeleteDocIds(server.mongoDB))).Methods(http.MethodDelete)
	router.Handle("/CloseDevice", AuthMiddleware(handlers.CloseDevice(server.mongoDB))).Methods(http.MethodDelete)
	router.Handle("/GetJwt", RefreshMiddleware(handlers.RefreshToken(server.mongoDB))).Methods(http.MethodGet)

	router.Handle("/register", handlers.Register(server.mongoDB)).Methods(http.MethodPost)
	router.Handle("/login", handlers.Login(server.mongoDB)).Methods(http.MethodPost)
	router.Handle("/loginForce", handlers.LoginForce(server.mongoDB)).Methods(http.MethodPost)

	return router
}
