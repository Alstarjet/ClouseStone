package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/clients"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// maxUploadBody es el tope del cuerpo de /UploadData (lotes de respaldo).
const maxUploadBody = 5 << 20 // 5 MB

// backupStore es lo que /UploadData necesita de la base. Lo cumple
// *database.MongoClient; las pruebas usan uno en memoria.
type backupStore interface {
	userFinder
	UpsertBackup(ctx context.Context, collection, usermongoid string, backupAt time.Time, ups []database.Upsert) error
}

// UploadDataSchedule respalda lo que manda el dispositivo: clientes, cargos,
// pedidos y pagos.
//
//   - Cada registro se lee por separado: uno que nunca se va a poder aceptar
//     (tipo inesperado, changedfields desconocido, sin uuid) se rechaza solo y lo
//     demás se guarda (I-02). Entonces responde 422 con la lista de rechazados.
//   - Cada colección se escribe en un solo viaje a Mongo, con upsert por
//     (usermongoid, uuid): repetir la subida no duplica nada (I-03, I-08).
//   - Un error de Mongo es 500: el dispositivo reintenta el lote completo.
//   - Ya no anota nada en la cola de los demás dispositivos (I-07).
//
// El contexto trae el tiempo límite propio de la ruta (route.go).
func UploadDataSchedule(db *database.MongoClient) http.Handler {
	return uploadData(db)
}

func uploadData(store backupStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
		ctx := r.Context()

		// El token siempre trae el dispositivo (AuthMiddleware); sin él no se respalda
		if _, ok := ctx.Value(ctxkeys.DeviceID).(string); !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("UploadDataSchedule: read body error: %v", err)
			http.Error(w, `{"error":"error reading request"}`, http.StatusBadRequest)
			return
		}
		// Sólo la forma general (un objeto con listas) puede rechazar el lote entero
		var request models.RequestUpdate
		if err = json.Unmarshal(body, &request); err != nil {
			log.Printf("UploadDataSchedule: unmarshal error: %v", err)
			http.Error(w, `{"error":"invalid request format"}`, http.StatusBadRequest)
			return
		}
		parts, rejected := decodeBackup(request)

		emailRequest, ok := ctx.Value(ctxkeys.Email).(string)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		user, err := store.FindUser(ctx, emailRequest)
		if err != nil {
			log.Printf("UploadDataSchedule: find user error: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		// La versión gratis no respalda en línea: 403 antes de escribir nada
		if !allowsCloudBackup(w, user) {
			return
		}

		// Una sola hora del servidor para todo el lote (backupat, base de /GetDataSince)
		backupAt := time.Now().UTC()
		for _, part := range parts {
			if err := store.UpsertBackup(ctx, part.collection, user.ID.Hex(), backupAt, part.upserts); err != nil {
				log.Printf("UploadDataSchedule: %v", err)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				return
			}
		}

		if len(rejected) > 0 {
			log.Printf("UploadDataSchedule: usuario %s: %d registros rechazados: %s", user.ID.Hex(), len(rejected), rejectedSummary(rejected))
			writeJSON(w, http.StatusUnprocessableEntity, models.BackupRejectedResponse{
				Error:      "rejected_records",
				Message:    rejectedMessage(len(rejected)),
				TypeClient: user.TypeClient,
				Rejected:   rejected,
			})
			return
		}
		writeJSON(w, http.StatusOK, models.BackupResponse{
			Message:    "Datos Respaldados con Éxito",
			Status:     200,
			TypeClient: user.TypeClient,
		})
	})
}

// backupPart son los documentos de una colección, listos para el upsert.
type backupPart struct {
	collection string
	upserts    []database.Upsert
}

// decodeBackup lee cada registro del lote por separado y regresa, por colección y
// en el orden en que llegaron, los que sí se pueden guardar, más los rechazados.
// No toca la base.
func decodeBackup(request models.RequestUpdate) ([]backupPart, []models.RejectedRecord) {
	var rejected []models.RejectedRecord
	parts := []backupPart{
		decodeRecords("clients", request.Clients, acceptClient, &rejected),
		decodeRecords("charges", request.Charges, acceptCharge, &rejected),
		decodeRecords("orders", request.Orders, acceptCharge, &rejected),
		decodeRecords("payments", request.Payments, acceptPayment, &rejected),
	}
	return parts, rejected
}

// decodeRecords decodifica los registros de una colección como T y los acepta o
// los rechaza uno por uno.
func decodeRecords[T any](collection string, raws []json.RawMessage, accept func(T) (database.Upsert, error), rejected *[]models.RejectedRecord) backupPart {
	part := backupPart{collection: collection}
	for _, raw := range raws {
		var record T
		err := json.Unmarshal(raw, &record)
		var up database.Upsert
		if err == nil {
			up, err = accept(record)
		}
		if err != nil {
			*rejected = append(*rejected, models.RejectedRecord{Store: collection, UUID: recordUUID(raw), Reason: rejectReason(err)})
			continue
		}
		part.upserts = append(part.upserts, up)
	}
	return part
}

// errNoUUID: sin uuid no hay forma de identificar el documento en la nube.
var errNoUUID = errors.New("sin uuid")

func acceptClient(client models.Client) (database.Upsert, error) {
	if client.UUID == "" {
		return database.Upsert{}, errNoUUID
	}
	if err := clients.Validate(client); err != nil {
		return database.Upsert{}, err
	}
	return clients.Upsert(client)
}

// acceptCharge sirve para cargos y pedidos (los pedidos usan el modelo Charge).
func acceptCharge(charge models.Charge) (database.Upsert, error) {
	if charge.UUID == "" {
		return database.Upsert{}, errNoUUID
	}
	return database.FullUpsert(charge.UUID, charge)
}

func acceptPayment(payment models.Payment) (database.Upsert, error) {
	if payment.UUID == "" {
		return database.Upsert{}, errNoUUID
	}
	return database.FullUpsert(payment.UUID, payment)
}

// recordUUID saca el uuid de un registro aunque no se haya podido leer completo,
// para que la app sepa cuál fue. Vacío si no trae uno de texto.
func recordUUID(raw json.RawMessage) string {
	var probe struct {
		UUID any `json:"uuid"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	uuid, _ := probe.UUID.(string)
	return uuid
}

// rejectReason explica en pocas palabras por qué no se aceptó un registro.
func rejectReason(err error) string {
	var typeErr *json.UnmarshalTypeError
	var timeErr *time.ParseError
	switch {
	case errors.As(err, &typeErr):
		if typeErr.Field == "" {
			return fmt.Sprintf("el registro llegó como %s y se esperaba un objeto", jsonValueName(typeErr.Value))
		}
		return fmt.Sprintf("tipo inesperado en %q: llegó %s y se esperaba %s",
			typeErr.Field, jsonValueName(typeErr.Value), goTypeName(typeErr.Type))
	case errors.As(err, &timeErr):
		return fmt.Sprintf("fecha inválida %q: se espera fecha y hora RFC 3339 (p. ej. 2026-11-11T10:00:00Z)", timeErr.Value)
	case errors.Is(err, errNoUUID), errors.Is(err, clients.ErrUnknownField), errors.Is(err, clients.ErrPhoneType):
		return err.Error()
	default:
		return "formato inválido: " + err.Error()
	}
}

// jsonValueName traduce el tipo JSON de un error de decodificación ("number
// 34.5", "string", "bool"…).
func jsonValueName(value string) string {
	kind, detail, _ := strings.Cut(value, " ")
	names := map[string]string{"string": "texto", "number": "número", "bool": "verdadero/falso",
		"object": "objeto", "array": "lista"}
	name, ok := names[kind]
	if !ok {
		return value
	}
	if detail != "" {
		return name + " " + detail
	}
	return name
}

// goTypeName dice qué tipo esperaba el campo.
func goTypeName(t reflect.Type) string {
	if t == nil {
		return "otro tipo"
	}
	switch t.Kind() {
	case reflect.String:
		return "texto"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "un número entero"
	case reflect.Float32, reflect.Float64:
		return "un número"
	case reflect.Bool:
		return "verdadero/falso"
	case reflect.Slice, reflect.Array:
		return "una lista"
	case reflect.Struct, reflect.Map:
		return "un objeto"
	default:
		return t.String()
	}
}

func rejectedMessage(n int) string {
	if n == 1 {
		return "Se respaldó todo menos 1 registro que el servidor no puede aceptar"
	}
	return fmt.Sprintf("Se respaldó todo menos %d registros que el servidor no puede aceptar", n)
}

// rejectedSummary resume los rechazos para el log (colección, uuid y motivo; nada
// de los datos del negocio).
func rejectedSummary(rejected []models.RejectedRecord) string {
	const maxLogged = 10
	var b strings.Builder
	for i, rec := range rejected {
		if i == maxLogged {
			fmt.Fprintf(&b, " … y %d más", len(rejected)-maxLogged)
			break
		}
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s/%s: %s", rec.Store, rec.UUID, rec.Reason)
	}
	return b.String()
}
