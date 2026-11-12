package mainservice

import (
	"financial-Assistant/internal/mainservice/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

// NewRouter crea un nuevo router Gorilla Mux y configura sus rutas. //nuevo comentario
func NewRouter(server *Server) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	// /UploadData y /login tienen tiempo límite propio (timelimit.go); las demás, los 15 s del servidor.
	router.Handle("/UploadData", withTimeLimit(uploadTimeLimit, AuthMiddleware(handlers.UploadDataSchedule(server.mongoDB)))).Methods(http.MethodPost)
	router.Handle("/GetDataSince", AuthMiddleware(handlers.GetDataSince(server.mongoDB))).Methods(http.MethodGet)
	// Legado (cola por dispositivo, que ya nadie escribe): la app de producción todavía los llama desde
	// una pantalla vieja. Se quitan cuando se despliegue el frontend sin ella.
	router.Handle("/GetData", AuthMiddleware(handlers.GetData(server.mongoDB))).Methods(http.MethodGet)
	router.Handle("/DeleteIds", AuthMiddleware(handlers.DeleteDocIds(server.mongoDB))).Methods(http.MethodDelete)
	router.Handle("/CloseDevice", AuthMiddleware(handlers.CloseDevice(server.mongoDB))).Methods(http.MethodDelete)
	router.Handle("/GetJwt", RefreshMiddleware(handlers.RefreshToken(server.mongoDB))).Methods(http.MethodGet)

	// Catálogo curado de productos y kits (solo lectura). Cualquier usuario autenticado.
	catalog := server.catalog
	router.Handle("/catalog/products", AuthMiddleware(handlers.CatalogProducts(catalog))).Methods(http.MethodGet)
	router.Handle("/catalog/products/{id}", AuthMiddleware(handlers.CatalogProduct(catalog))).Methods(http.MethodGet)
	router.Handle("/catalog/kits", AuthMiddleware(handlers.CatalogKits(catalog))).Methods(http.MethodGet)
	router.Handle("/catalog/kits/{slug}", AuthMiddleware(handlers.CatalogKit(catalog))).Methods(http.MethodGet)

	router.Handle("/register", handlers.Register(server.mongoDB)).Methods(http.MethodPost)
	router.Handle("/login", withTimeLimit(loginTimeLimit, handlers.Login(server.mongoDB))).Methods(http.MethodPost)
	// Obsoleto: igual que /login (ya no hay límite de dispositivos); se conserva para versiones viejas de la app.
	router.Handle("/loginForce", withTimeLimit(loginTimeLimit, handlers.LoginForce(server.mongoDB))).Methods(http.MethodPost)

	return router
}
