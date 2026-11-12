package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// validBackup tiene la forma del respaldo que manda la PWA a /UploadData: un
// documento de cada colección con los tipos que espera Go.
const validBackup = `{
	"clients": [{"uuid":"cli-1","name":"Ana","lastname":"López","age":40,"city":"Puebla",
		"neighborhood":"Centro","address":"5 de Mayo 10","phone":"2221234567","daywork":"lunes",
		"createat":"2026-10-20T15:04:05Z","updateat":"2026-10-20T15:04:05Z","status":"active"}],
	"charges": [{"uuid":"car-1","clientuuid":"cli-1","clientname":"Ana López",
		"products":[{"uuid":"prod-1","key":"S1122","name":"Crema","page":"12","price":150.5,"type":"pieza","catalog":"Shelo","quantity":2,"total":301}],
		"discount":0,"subtotal":301,"finalprice":301,
		"createat":"2026-10-20T15:05:00Z","updateat":"2026-10-20T15:05:00Z","status":"active"}],
	"orders": [{"uuid":"ped-1","clientuuid":"cli-1","clientname":"Ana López",
		"products":[{"uuid":"prod-2","key":"S2040","name":"Jabón","page":"30","price":80,"type":"pieza","catalog":"Shelo","quantity":1,"total":80}],
		"discount":0,"subtotal":80,"finalprice":80,
		"createat":"2026-10-20T15:06:00Z","updateat":"2026-10-20T15:06:00Z","status":"active"}],
	"payments": [{"uuid":"pag-1","clientuuid":"cli-1","clientname":"Ana López","amount":150,"method":"efectivo",
		"concept":"abono","createat":"2026-10-20T15:07:00Z","updateat":"2026-10-20T15:07:00Z","status":"active"}],
	"deviceid": "dispositivo-1"
}`

const (
	testEmail    = "ana@ejemplo.mx"
	testDeviceID = "dispositivo-1"
)

// goodUUIDs son los documentos de validBackup, por colección.
var goodUUIDs = map[string]string{"clients": "cli-1", "charges": "car-1", "orders": "ped-1", "payments": "pag-1"}

// uploadRequest arma la petición a /UploadData con el email y el dispositivo que
// pone AuthMiddleware en el contexto ("" = sin ese valor).
func uploadRequest(email, deviceID, body string) *http.Request {
	ctx := context.Background()
	if email != "" {
		ctx = context.WithValue(ctx, ctxkeys.Email, email)
	}
	if deviceID != "" {
		ctx = context.WithValue(ctx, ctxkeys.DeviceID, deviceID)
	}
	return httptest.NewRequest(http.MethodPost, "/UploadData", strings.NewReader(body)).WithContext(ctx)
}

// serveUpload manda body a /UploadData. Con db nil, si el handler intentara leer o
// escribir en Mongo la prueba falla en vez de entrar en pánico.
func serveUpload(t *testing.T, db *database.MongoClient, email, deviceID, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("el handler intentó usar la base de datos: %v", r)
		}
	}()
	UploadDataSchedule(db).ServeHTTP(rec, uploadRequest(email, deviceID, body))
	return rec
}

// fakeStore es la base en memoria de las pruebas de /UploadData: anota lo que le
// piden escribir y puede fallar a propósito.
type fakeStore struct {
	user      models.User
	findErr   error
	upsertErr error

	writes    map[string][]database.Upsert // colección → documentos
	owners    map[string]bool              // usermongoid de cada escritura
	backupAts map[time.Time]bool
}

func (f *fakeStore) FindUser(_ context.Context, email string) (models.User, error) {
	if f.findErr != nil {
		return models.User{}, f.findErr
	}
	if email != f.user.Email {
		return models.User{}, errors.New("no existe")
	}
	return f.user, nil
}

func (f *fakeStore) UpsertBackup(_ context.Context, collection, usermongoid string, backupAt time.Time, ups []database.Upsert) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	if f.writes == nil {
		f.writes, f.owners, f.backupAts = map[string][]database.Upsert{}, map[string]bool{}, map[time.Time]bool{}
	}
	if len(ups) > 0 {
		f.writes[collection] = append(f.writes[collection], ups...)
		f.owners[usermongoid] = true
		f.backupAts[backupAt] = true
	}
	return nil
}

// written regresa los uuid que se escribieron en cada colección.
func (f *fakeStore) written() map[string][]string {
	out := map[string][]string{}
	for coll, ups := range f.writes {
		for _, up := range ups {
			out[coll] = append(out[coll], up.UUID)
		}
	}
	return out
}

func newFakeStore(typeclient string) *fakeStore {
	return &fakeStore{user: models.User{ID: primitive.NewObjectID(), Email: testEmail, TypeClient: typeclient}}
}

func serveFake(store *fakeStore, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	uploadData(store).ServeHTTP(rec, uploadRequest(testEmail, testDeviceID, body))
	return rec
}

// withBad agrega un registro al principio de una colección de validBackup.
func withBad(collection, record string) string {
	return strings.Replace(validBackup, `"`+collection+`": [`, `"`+collection+`": [`+record+`,`, 1)
}

// Sólo la forma general del cuerpo rechaza el lote entero con 400, antes de
// buscar al usuario o guardar nada.
func TestUploadDataRejectsMalformedBody(t *testing.T) {
	const (
		errFormat = `{"error":"invalid request format"}`
		errRead   = `{"error":"error reading request"}`
	)
	tests := []struct {
		name     string
		body     string
		wantBody string
	}{
		{"cuerpo vacío", "", errFormat},
		{"JSON incompleto", `{"clients":[`, errFormat},
		{"no es un objeto", `[{"uuid":"cli-1"}]`, errFormat},
		{"clients no es una lista", `{"clients":{"uuid":"cli-1"}}`, errFormat},
		{"cuerpo de más de 5 MB", `{"clients":[{"name":"` + strings.Repeat("a", 5<<20) + `"}]}`, errRead},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveUpload(t, nil, testEmail, testDeviceID, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("código = %d; se esperaba 400 (cuerpo: %s)", rec.Code, rec.Body)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
				t.Errorf("cuerpo = %s; se esperaba %s", got, tt.wantBody)
			}
		})
	}
}

// I-02: un registro que nunca se va a poder aceptar no tumba a los demás. Se
// guarda todo lo demás y responde 422 con los rechazados.
func TestUploadDataRejectsOnlyBadRecords(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStore  string
		wantUUID   string
		wantReason string
	}{
		{"edad como texto", withBad("clients", `{"uuid":"cli-2","name":"Luis","age":"40"}`), "clients", "cli-2", `"age"`},
		{"edad con decimales", withBad("clients", `{"uuid":"cli-2","age":34.5}`), "clients", "cli-2", "número 34.5"},
		{"fecha sin hora (el uuid viene después)", withBad("clients", `{"createat":"2026-10-20","uuid":"cli-2"}`), "clients", "cli-2", "fecha inválida"},
		{"changedfields desconocido", withBad("clients", `{"uuid":"cli-2","name":"Luis","changedfields":["lastname","color"]}`), "clients", "cli-2", `"color"`},
		{"teléfono que es un objeto", withBad("clients", `{"uuid":"cli-2","phone":{"$x":1}}`), "clients", "cli-2", "phone"},
		{"cliente sin uuid", withBad("clients", `{"name":"Luis"}`), "clients", "", "sin uuid"},
		{"uuid numérico", withBad("clients", `{"uuid":5,"name":"Luis"}`), "clients", "", `"uuid"`},
		{"verdadero/falso como clave de un producto del cargo", withBad("charges", `{"uuid":"car-2","products":[{"key":true}]}`), "charges", "car-2", `"products.key"`},
		{"pedido sin uuid", withBad("orders", `{"clientuuid":"cli-1"}`), "orders", "", "sin uuid"},
		{"pedido null", withBad("orders", `null`), "orders", "", "sin uuid"},
		{"monto del pago como texto", withBad("payments", `{"uuid":"pag-2","amount":"150"}`), "payments", "pag-2", `"amount"`},
		{"un pago que no es un objeto", withBad("payments", `"pag-2"`), "payments", "", "el registro llegó como texto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore("Amethyst")
			rec := serveFake(store, tt.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("código = %d; se esperaba 422 (cuerpo: %s)", rec.Code, rec.Body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			var resp models.BackupRejectedResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("respuesta no es JSON: %v (%s)", err, rec.Body)
			}
			if resp.Error != "rejected_records" || resp.TypeClient != "Amethyst" || resp.Message == "" {
				t.Errorf("respuesta = %+v", resp)
			}
			if len(resp.Rejected) != 1 {
				t.Fatalf("rechazados = %+v; se esperaba uno", resp.Rejected)
			}
			got := resp.Rejected[0]
			if got.Store != tt.wantStore || got.UUID != tt.wantUUID || !strings.Contains(got.Reason, tt.wantReason) {
				t.Errorf("rechazado = %+v; se esperaba %s/%q con %q en el motivo", got, tt.wantStore, tt.wantUUID, tt.wantReason)
			}
			// Lo demás del lote sí se guardó
			for coll, uuid := range goodUUIDs {
				if w := store.written()[coll]; !reflect.DeepEqual(w, []string{uuid}) {
					t.Errorf("%s guardados = %v; se esperaba [%s]", coll, w, uuid)
				}
			}
		})
	}
}

// El 422 trae todos los rechazados, en orden, con las claves acordadas con la app.
func TestUploadDataRejectedBody(t *testing.T) {
	body := strings.Replace(withBad("clients", `{"uuid":"cli-2","age":"40"}`),
		`"payments": [`, `"payments": [{"uuid":"pag-2","amount":"150"},`, 1)
	rec := serveFake(newFakeStore("Amethyst"), body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("código = %d (cuerpo: %s)", rec.Code, rec.Body)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	for _, key := range []string{"error", "message", "typeclient", "rejected"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("falta %q en %v", key, resp)
		}
	}
	if len(resp) != 4 {
		t.Errorf("respuesta = %v; se esperaban sólo error, message, typeclient y rejected", resp)
	}
	rejected, _ := resp["rejected"].([]any)
	if len(rejected) != 2 {
		t.Fatalf("rechazados = %v; se esperaban 2", rejected)
	}
	first, _ := rejected[0].(map[string]any)
	second, _ := rejected[1].(map[string]any)
	if first["store"] != "clients" || first["uuid"] != "cli-2" || second["store"] != "payments" || second["uuid"] != "pag-2" {
		t.Errorf("rechazados = %v; se esperaba clients/cli-2 y luego payments/pag-2", rejected)
	}
	for _, r := range []map[string]any{first, second} {
		if len(r) != 3 || r["reason"] == "" {
			t.Errorf("rechazado = %v; se esperaban store, uuid y reason", r)
		}
	}
	if !strings.Contains(resp["message"].(string), "2 registros") {
		t.Errorf("message = %v", resp["message"])
	}
}

// Todo bien: 200 con el cuerpo de siempre, un solo dueño y una sola hora del
// servidor para todo el lote, y sólo en las cuatro colecciones del respaldo (ya no
// se anota nada en la cola de los demás dispositivos, I-07).
func TestUploadDataSavesEverything(t *testing.T) {
	store := newFakeStore("Amethyst")
	before := time.Now().UTC()
	rec := serveFake(store, validBackup)
	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d; se esperaba 200 (cuerpo: %s)", rec.Code, rec.Body)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("respuesta no es JSON: %v (%s)", err, rec.Body)
	}
	// ⚠️ El estado viaja con la clave "reports", no "status" (models.BackupResponse).
	want := map[string]any{"message": "Datos Respaldados con Éxito", "reports": float64(200), "typeclient": "Amethyst"}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("respuesta = %v; se esperaba %v", resp, want)
	}
	written := store.written()
	if len(written) != len(goodUUIDs) {
		t.Errorf("colecciones escritas = %v; se esperaban sólo %v", written, goodUUIDs)
	}
	for coll, uuid := range goodUUIDs {
		if !reflect.DeepEqual(written[coll], []string{uuid}) {
			t.Errorf("%s = %v; se esperaba [%s]", coll, written[coll], uuid)
		}
	}
	if !reflect.DeepEqual(store.owners, map[string]bool{store.user.ID.Hex(): true}) {
		t.Errorf("dueños = %v; se esperaba sólo %s", store.owners, store.user.ID.Hex())
	}
	if len(store.backupAts) != 1 {
		t.Fatalf("horas del servidor = %v; se esperaba una para todo el lote", store.backupAts)
	}
	for at := range store.backupAts {
		if at.Location() != time.UTC || at.Before(before) || at.After(time.Now().UTC()) {
			t.Errorf("backupat = %v; se esperaba la hora UTC del servidor", at)
		}
	}
	// El renglón de producto conserva su uuid
	charge := store.writes["charges"][0].Set
	products, _ := charge["products"].(bson.A)
	if row, _ := products[0].(bson.M); len(products) != 1 || row["uuid"] != "prod-1" {
		t.Errorf("renglones del cargo = %v", charge["products"])
	}
}

// Un número donde se espera texto se acepta y se guarda como texto (celdas de
// Excel, productos de la versión 1.0), en vez de rechazar el registro.
func TestUploadDataAcceptsNumbersAsText(t *testing.T) {
	body := `{"clients":[{"uuid":"cli-1","name":"Ana","lastname":"López","city":72000,"neighborhood":15,
		"address":100,"daywork":5,"phone":2221234567}],
		"charges":[{"uuid":"car-1","products":[{"uuid":"prod-1","key":1122,"page":12,"name":"Crema","price":150,"quantity":1,"total":150}]}]}`
	store := newFakeStore("Amethyst")
	rec := serveFake(store, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d; se esperaba 200 (cuerpo: %s)", rec.Code, rec.Body)
	}
	client := store.writes["clients"][0].Set
	for field, want := range map[string]any{"city": "72000", "neighborhood": "15", "address": "100", "daywork": "5",
		"phone": float64(2221234567)} {
		if client[field] != want {
			t.Errorf("cliente %q = %#v; se esperaba %#v", field, client[field], want)
		}
	}
	products, _ := store.writes["charges"][0].Set["products"].(bson.A)
	row, _ := products[0].(bson.M)
	if row["key"] != "1122" || row["page"] != "12" {
		t.Errorf("renglón = %v; se esperaba clave y página como texto", row)
	}
}

// La versión gratis no respalda: 403 y nada escrito, aunque traiga rechazados.
func TestUploadDataFreePlanWritesNothing(t *testing.T) {
	for _, body := range []string{validBackup, withBad("clients", `{"uuid":"cli-2","age":"40"}`)} {
		store := newFakeStore("Quartz")
		rec := serveFake(store, body)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("código = %d; se esperaba 403 (cuerpo: %s)", rec.Code, rec.Body)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"plan_without_backup"}` {
			t.Errorf("cuerpo = %s", got)
		}
		if len(store.writes) != 0 {
			t.Errorf("se escribió %v", store.written())
		}
	}
}

// Un error de Mongo es 500 (nunca 200 ni 422): el teléfono reintenta el lote
// completo, y repetirlo no duplica nada.
func TestUploadDataStoreErrorIs500(t *testing.T) {
	withRejected := withBad("clients", `{"uuid":"cli-2","age":"40"}`)
	tests := []struct {
		name  string
		store *fakeStore
		body  string
	}{
		{"no se pudo leer al usuario", &fakeStore{findErr: errors.New("sin conexión")}, validBackup},
		{"no se pudo escribir", func() *fakeStore { s := newFakeStore("Amethyst"); s.upsertErr = errors.New("sin conexión"); return s }(), validBackup},
		{"no se pudo escribir y había rechazados", func() *fakeStore { s := newFakeStore("Amethyst"); s.upsertErr = errors.New("sin conexión"); return s }(), withRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveFake(tt.store, tt.body)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("código = %d; se esperaba 500 (cuerpo: %s)", rec.Code, rec.Body)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"internal server error"}` {
				t.Errorf("cuerpo = %s", got)
			}
		})
	}
}

// Si AuthMiddleware no dejó el email o el dispositivo en el contexto, el
// handler responde 401 sin tocar la base.
func TestUploadDataRequiresContext(t *testing.T) {
	tests := []struct {
		name            string
		email, deviceID string
	}{
		{"sin dispositivo", testEmail, ""},
		{"sin email", "", testDeviceID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveUpload(t, nil, tt.email, tt.deviceID, validBackup)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("código = %d; se esperaba 401 (cuerpo: %s)", rec.Code, rec.Body)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"unauthorized"}` {
				t.Errorf("cuerpo = %s", got)
			}
		})
	}
}

// Lo que sí acepta la lectura del respaldo, registro por registro.
func TestDecodeBackup(t *testing.T) {
	decode := func(t *testing.T, body string) ([]backupPart, []models.RejectedRecord) {
		t.Helper()
		var request models.RequestUpdate
		if err := json.Unmarshal([]byte(body), &request); err != nil {
			t.Fatalf("cuerpo inválido: %v", err)
		}
		return decodeBackup(request)
	}
	// first regresa los campos del primer documento de la colección.
	first := func(t *testing.T, parts []backupPart, collection string) bson.M {
		t.Helper()
		for _, p := range parts {
			if p.collection == collection && len(p.upserts) > 0 {
				return p.upserts[0].Set
			}
		}
		t.Fatalf("no se aceptó ningún documento de %s: %+v", collection, parts)
		return nil
	}

	parts, rejected := decode(t, validBackup)
	if len(rejected) != 0 {
		t.Fatalf("rechazados = %+v", rejected)
	}
	order := []string{}
	for _, p := range parts {
		order = append(order, p.collection)
	}
	if !reflect.DeepEqual(order, []string{"clients", "charges", "orders", "payments"}) {
		t.Errorf("orden = %v; un cliente nuevo sube antes que sus movimientos", order)
	}

	tests := []struct {
		name       string
		body       string
		collection string
		check      func(bson.M) bool
	}{
		{"teléfono como número", `{"clients":[{"uuid":"c","phone":2221234567}]}`, "clients",
			func(d bson.M) bool { return d["phone"] == float64(2221234567) }},
		{"teléfono como texto", `{"clients":[{"uuid":"c","phone":"222 123 4567"}]}`, "clients",
			func(d bson.M) bool { return d["phone"] == "222 123 4567" }},
		{"edad ausente queda en 0", `{"clients":[{"uuid":"c"}]}`, "clients",
			func(d bson.M) bool { return d["age"] == int32(0) }},
		{"edad null queda en 0", `{"clients":[{"uuid":"c","age":null}]}`, "clients",
			func(d bson.M) bool { return d["age"] == int32(0) }},
		{"monto entero", `{"payments":[{"uuid":"p","amount":150}]}`, "payments",
			func(d bson.M) bool { return d["amount"] == 150.0 }},
		{"fecha con zona horaria", `{"payments":[{"uuid":"p","createat":"2026-10-20T09:07:00-06:00"}]}`, "payments",
			func(d bson.M) bool {
				dt, ok := d["createat"].(primitive.DateTime)
				return ok && dt.Time().UTC().Hour() == 15
			}},
		{"tipo de cliente", `{"clients":[{"uuid":"c","type":"blacklist"}]}`, "clients",
			func(d bson.M) bool { return d["type"] == "blacklist" }},
		{"campos desconocidos se ignoran", `{"clients":[{"uuid":"c","color":"rojo"}],"extra":true}`, "clients",
			func(d bson.M) bool { _, ok := d["color"]; return !ok }},
		{"products y deviceid de cualquier forma no importan", `{"clients":[{"uuid":"c"}],"products":"x","deviceid":5}`, "clients",
			func(d bson.M) bool { return d["status"] == "" }},
		{"backupat del dispositivo se ignora", `{"charges":[{"uuid":"car","backupat":"1999-01-01T00:00:00Z"}]}`, "charges",
			func(d bson.M) bool { _, ok := d["backupat"]; return !ok }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, rejected := decode(t, tt.body)
			if len(rejected) != 0 {
				t.Fatalf("rechazados = %+v", rejected)
			}
			if doc := first(t, parts, tt.collection); !tt.check(doc) {
				t.Errorf("guardado = %v", doc)
			}
		})
	}

	t.Run("colecciones ausentes", func(t *testing.T) {
		parts, rejected := decode(t, `{}`)
		for _, p := range parts {
			if len(p.upserts) != 0 {
				t.Errorf("%s = %v", p.collection, p.upserts)
			}
		}
		if len(rejected) != 0 {
			t.Errorf("rechazados = %v", rejected)
		}
	})
	t.Run("el mismo documento dos veces: en el orden en que llegó", func(t *testing.T) {
		parts, _ := decode(t, `{"clients":[{"uuid":"c","name":"Ana"},{"uuid":"c","name":"Ana María"}]}`)
		ups := parts[0].upserts
		if len(ups) != 2 || ups[0].Set["name"] != "Ana" || ups[1].Set["name"] != "Ana María" {
			t.Errorf("upserts = %+v", ups)
		}
	})
}
