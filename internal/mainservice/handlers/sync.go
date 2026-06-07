package handlers

import (
	"context"
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"io"
	"log"
	"net/http"
	"time"
)

const maxSyncBodySize = 10 << 20 // 10MB

// Sync implementa la sincronización v2 (delta por timestamp + soft delete).
//
// En una sola llamada el dispositivo sube sus cambios locales y recibe todo lo
// que cambió en el servidor desde su último cursor (req.Since). El servidor
// sella el tiempo una sola vez por petición y lo devuelve como nuevo cursor.
func Sync(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxSyncBodySize)
		ctx := r.Context()

		emailRequest, ok := ctx.Value(ctxkeys.Email).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		user, err := db.FindUser(emailRequest)
		if err != nil {
			log.Printf("Sync: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		var req models.SyncRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Sync: read body error: %v", err)
			http.Error(w, `{"error":"error reading request"}`, http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			log.Printf("Sync: unmarshal error: %v", err)
			http.Error(w, `{"error":"invalid request format"}`, http.StatusBadRequest)
			return
		}

		usermongoid := user.ID.Hex()
		// Un único sello de tiempo por petición: se usa tanto para marcar los
		// upserts como, indirectamente, como base del nuevo cursor. Truncado a
		// milisegundos para coincidir con la precisión de almacenamiento de Mongo.
		serverTime := time.Now().UTC().Truncate(time.Millisecond)

		// --- PUSH: aplicar los cambios locales del dispositivo ---
		if err := pushChanges(ctx, db, usermongoid, req.Changes, serverTime); err != nil {
			log.Printf("Sync: push error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		// --- PULL: devolver lo que cambió en el servidor desde req.Since ---
		changes, err := pullChanges(ctx, db, usermongoid, req.Since)
		if err != nil {
			log.Printf("Sync: pull error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		response := models.SyncResponse{
			ServerTime: serverTime,
			Changes:    changes,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Sync: encode response error: %v", err)
		}
	})
}

// pushChanges hace upsert (por uuid) de cada documento entrante, sellando
// updateat con serverTime. Un Status="deleted" se persiste como tombstone.
func pushChanges(ctx context.Context, db *database.MongoClient, usermongoid string, changes models.SyncChanges, serverTime time.Time) error {
	for _, c := range changes.Clients {
		if err := db.UpsertClientSync(ctx, usermongoid, c, serverTime); err != nil {
			return err
		}
	}
	for _, c := range changes.Charges {
		if err := db.UpsertChargeSync(ctx, usermongoid, c, serverTime); err != nil {
			return err
		}
	}
	for _, o := range changes.Orders {
		if err := db.UpsertOrderSync(ctx, usermongoid, o, serverTime); err != nil {
			return err
		}
	}
	for _, p := range changes.Payments {
		if err := db.UpsertPaymentSync(ctx, usermongoid, p, serverTime); err != nil {
			return err
		}
	}
	for _, p := range changes.Products {
		if err := db.UpsertProductSync(ctx, usermongoid, p, serverTime); err != nil {
			return err
		}
	}
	return nil
}

// pullChanges devuelve todo lo que cambió en el servidor desde since (snapshot
// completo de activos si since es nil).
func pullChanges(ctx context.Context, db *database.MongoClient, usermongoid string, since *time.Time) (models.SyncChanges, error) {
	var out models.SyncChanges
	var err error

	if out.Clients, err = db.FindClientsChangedSince(ctx, usermongoid, since); err != nil {
		return out, err
	}
	if out.Charges, err = db.FindChargesChangedSince(ctx, usermongoid, since); err != nil {
		return out, err
	}
	if out.Orders, err = db.FindOrdersChangedSince(ctx, usermongoid, since); err != nil {
		return out, err
	}
	if out.Payments, err = db.FindPaymentsChangedSince(ctx, usermongoid, since); err != nil {
		return out, err
	}
	if out.Products, err = db.FindProductsChangedSince(ctx, usermongoid, since); err != nil {
		return out, err
	}
	return out, nil
}
