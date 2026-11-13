package main

import (
	"net/http"

	"github.com/klauspost/compress/gzhttp"
	"github.com/rs/cors"
)

// corsMaxAge son los segundos que el navegador guarda la respuesta a la pregunta
// previa de CORS (OPTIONS) de cada ruta. Sin ella, cada llamada a la API iba
// precedida de un OPTIONS: la mitad de las peticiones del servicio entre octubre
// y noviembre de 2026. Cada navegador aplica su propio tope (Chrome, 2 horas).
const corsMaxAge = 7200

// newHandler arma lo que envuelve a las rutas:
//   - CORS: sólo el origen del frontend (URL_FRONT), con la respuesta del OPTIONS
//     en caché del navegador.
//   - Compresión gzip de las respuestas de 1 KB o más cuando el cliente la acepta
//     (los navegadores lo hacen solos). El JSON de /login, que trae la cuenta
//     completa, baja a una fracción. Las respuestas chicas, como la de /GetJwt
//     con el token nuevo, salen igual que antes.
//
// La envoltura de gzhttp deja llegar a la conexión real (Unwrap, desde la v1.18
// de klauspost/compress). Sin eso, los plazos propios de /UploadData y /login
// (withTimeLimit, con http.ResponseController) dejarían de aplicarse sin dar
// error y volvería I-03.
func newHandler(router http.Handler, frontOrigin string) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{frontOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Refresh-Token"},
		AllowCredentials: true,
		MaxAge:           corsMaxAge,
	})
	return c.Handler(gzhttp.GzipHandler(router))
}
