package handlers

import (
	"context"
	"log"
	"net/http"
	"time"
)

// planExpirer es lo que ExpirePlans necesita de la base (database.MongoClient).
type planExpirer interface {
	ExpirePaidPlans(ctx context.Context, now time.Time) (int64, error)
}

// ExpirePlansResponse es la respuesta de /internal/plans/expire.
type ExpirePlansResponse struct {
	// Cuántas cuentas pasaron a la versión gratis en esta corrida.
	Switched int64 `json:"switched"`
}

// ExpirePlans pasa a la versión gratis las cuentas de pago que ya vencieron. La llama Cloud
// Scheduler (una vez por semana; ver SchedulerMiddleware): quien vence sigue respaldando hasta
// la siguiente corrida, y esos días no se cobran. Correrla dos veces no hace daño.
func ExpirePlans(store planExpirer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switched, err := store.ExpirePaidPlans(r.Context(), time.Now())
		if err != nil {
			log.Printf("ExpirePlans: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "expire_failed")
			return
		}
		writeJSON(w, http.StatusOK, ExpirePlansResponse{Switched: switched})
	})
}
