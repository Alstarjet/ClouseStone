package mainservice

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
)

// Tiempos límite propios, en lugar de los 15 s de lectura y escritura del
// servidor (main.go). Con esos 15 s un lote que tardaba más se guardaba, pero la
// respuesta ya no salía: el teléfono lo daba por fallido y lo repetía para
// siempre (I-03).
//   - /UploadData recibe lotes de hasta 5 MB, que con una red de celular lenta
//     pueden tardar en llegar.
//   - /login (y /loginForce) regresan todos los datos de la cuenta, que pueden
//     tardar en bajar.
const (
	uploadTimeLimit = 60 * time.Second
	loginTimeLimit  = 60 * time.Second
	// writeGrace es el margen, ya vencido el tiempo límite, para mandar la
	// respuesta: al vencer se cancelan las consultas a Mongo y el 500 todavía
	// alcanza a salir.
	writeGrace = 5 * time.Second
)

// withTimeLimit le da a una ruta su propio tiempo límite: alarga el plazo de
// lectura del cuerpo y el de escritura de la respuesta en esa conexión (las
// demás rutas siguen con los del servidor) y pone el mismo plazo en el contexto
// de la petición, con el que el handler consulta y escribe en Mongo.
func withTimeLimit(limit time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline := time.Now().Add(limit)
		rc := http.NewResponseController(w)
		// Sin conexión real (p. ej. httptest.ResponseRecorder) no hay plazos que alargar
		if err := rc.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
			log.Printf("withTimeLimit: plazo de lectura: %v", err)
		}
		if err := rc.SetWriteDeadline(deadline.Add(writeGrace)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			log.Printf("withTimeLimit: plazo de escritura: %v", err)
		}
		ctx, cancel := context.WithDeadline(r.Context(), deadline)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
