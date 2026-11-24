package mainservice

import (
	"context"
	"financial-Assistant/internal/mainservice/moduls/googleid"
	"log"
	"net/http"
	"os"
	"strings"
)

// Rutas internas (/internal/*) que sólo llama Cloud Scheduler. Cloud Run es público, así que
// el backend comprueba él mismo quién llama: un token de identidad (OIDC) de Google, emitido
// para la cuenta de servicio del job y con la URL de la ruta como audiencia.
//
//   - SCHEDULER_SERVICE_ACCOUNT: correo de la cuenta de servicio del job.
//   - SCHEDULER_AUDIENCE: la audiencia que se configuró en el job (la URL completa de la ruta).
//
// Sin alguna de las dos, la ruta responde 503 y no hace nada.

type idTokenVerifier interface {
	Verify(ctx context.Context, raw, audience string) (*googleid.Claims, error)
}

var schedulerVerifier idTokenVerifier = googleid.NewVerifier(googleid.CertsURL, nil)

// SchedulerMiddleware deja pasar sólo a Cloud Scheduler (lee la configuración en cada llamada,
// igual que AuthMiddleware con ACCESS_SECRET).
func SchedulerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		schedulerAuth(schedulerVerifier, os.Getenv("SCHEDULER_AUDIENCE"), os.Getenv("SCHEDULER_SERVICE_ACCOUNT"), next).ServeHTTP(w, r)
	})
}

func schedulerAuth(verifier idTokenVerifier, audience, account string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if audience == "" || account == "" {
			log.Printf("SchedulerMiddleware: falta SCHEDULER_AUDIENCE o SCHEDULER_SERVICE_ACCOUNT; %s no hace nada", r.URL.Path)
			http.Error(w, `{"error":"scheduler_not_configured"}`, http.StatusServiceUnavailable)
			return
		}
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}
		claims, err := verifier.Verify(r.Context(), raw, audience)
		if err != nil {
			log.Printf("SchedulerMiddleware: %s: token rechazado: %v", r.URL.Path, err)
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		if !claims.EmailVerified || !strings.EqualFold(claims.Email, account) {
			log.Printf("SchedulerMiddleware: %s: cuenta no autorizada %q", r.URL.Path, claims.Email)
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
