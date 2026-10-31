package handlers

import (
	"context"
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// validBackup tiene la forma del respaldo que manda la PWA a /UploadData: un
// documento de cada colección con los tipos que espera Go.
const validBackup = `{
	"clients": [{"uuid":"cli-1","name":"Ana","lastname":"López","age":40,"city":"Puebla",
		"neighborhood":"Centro","address":"5 de Mayo 10","phone":"2221234567","daywork":"lunes",
		"createat":"2026-10-20T15:04:05Z","updateat":"2026-10-20T15:04:05Z","status":"active"}],
	"charges": [{"uuid":"car-1","clientuuid":"cli-1","clientname":"Ana López",
		"products":[{"key":"S1122","name":"Crema","page":"12","price":150.5,"type":"pieza","catalog":"Shelo","quantity":2,"total":301}],
		"discount":0,"subtotal":301,"finalprice":301,
		"createat":"2026-10-20T15:05:00Z","updateat":"2026-10-20T15:05:00Z","status":"active"}],
	"orders": [{"uuid":"ped-1","clientuuid":"cli-1","clientname":"Ana López",
		"products":[{"key":"S2040","name":"Jabón","page":"30","price":80,"type":"pieza","catalog":"Shelo","quantity":1,"total":80}],
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

// serveUpload manda body a /UploadData con el email y el dispositivo que pone
// AuthMiddleware en el contexto ("" = sin ese valor). Con db nil, si el handler
// intentara leer o escribir en Mongo la prueba falla en vez de entrar en pánico.
func serveUpload(t *testing.T, db *database.MongoClient, email, deviceID, body string) *httptest.ResponseRecorder {
	t.Helper()
	ctx := context.Background()
	if email != "" {
		ctx = context.WithValue(ctx, ctxkeys.Email, email)
	}
	if deviceID != "" {
		ctx = context.WithValue(ctx, ctxkeys.DeviceID, deviceID)
	}
	req := httptest.NewRequest(http.MethodPost, "/UploadData", strings.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("el handler intentó usar la base de datos: %v", r)
		}
	}()
	UploadDataSchedule(db).ServeHTTP(rec, req)
	return rec
}

// Un solo campo con otro tipo hace que Go rechace TODO el respaldo con 400,
// antes de buscar al usuario o guardar cualquier documento.
func TestUploadDataRejectsWrongTypes(t *testing.T) {
	const (
		errFormat = `{"error":"invalid request format"}`
		errRead   = `{"error":"error reading request"}`
	)
	oneBadClient := strings.Replace(validBackup, `"clients": [`, `"clients": [{"uuid":"cli-2","name":"Luis","age":"40"},`, 1)

	tests := []struct {
		name     string
		body     string
		wantBody string
	}{
		{"edad como texto", `{"clients":[{"uuid":"cli-1","age":"40"}]}`, errFormat},
		{"edad con decimales", `{"clients":[{"uuid":"cli-1","age":34.5}]}`, errFormat},
		{"monto del pago como texto", `{"payments":[{"uuid":"pag-1","amount":"150"}]}`, errFormat},
		{"clave numérica en un producto del cargo", `{"charges":[{"uuid":"car-1","products":[{"key":1122,"page":"12"}]}]}`, errFormat},
		{"página numérica en un producto del cargo", `{"charges":[{"uuid":"car-1","products":[{"key":"S1122","page":12}]}]}`, errFormat},
		{"clave numérica en un producto del pedido", `{"orders":[{"uuid":"ped-1","products":[{"key":1122}]}]}`, errFormat},
		{"un cliente malo entre documentos válidos", oneBadClient, errFormat},
		{"fecha sin hora (no es RFC 3339)", `{"clients":[{"uuid":"cli-1","createat":"2026-10-20"}]}`, errFormat},
		{"cuerpo vacío", "", errFormat},
		{"JSON incompleto", `{"clients":[`, errFormat},
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

// Un changedfields con un campo que no se puede editar rechaza TODO el respaldo
// con 400 antes de tocar la base: nunca se responde 200 sin haber guardado.
func TestUploadDataRejectsUnknownChangedFields(t *testing.T) {
	oneBadClient := strings.Replace(validBackup, `"clients": [`,
		`"clients": [{"uuid":"cli-2","name":"Luis","changedfields":["lastname","color"]},`, 1)

	tests := []struct {
		name string
		body string
	}{
		{"campo que no existe", `{"clients":[{"uuid":"cli-1","changedfields":["color"]}]}`},
		{"uuid", `{"clients":[{"uuid":"cli-1","changedfields":["uuid"]}]}`},
		{"fecha de alta", `{"clients":[{"uuid":"cli-1","changedfields":["createat"]}]}`},
		{"un cliente malo entre documentos válidos", oneBadClient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveUpload(t, nil, testEmail, testDeviceID, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("código = %d; se esperaba 400 (cuerpo: %s)", rec.Code, rec.Body)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"invalid changedfields"}` {
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

// Lo que sí acepta la decodificación del respaldo (la misma que usa el handler).
func TestRequestUpdateDecoding(t *testing.T) {
	var full models.RequestUpdate
	if err := json.Unmarshal([]byte(validBackup), &full); err != nil {
		t.Fatalf("el respaldo válido no se decodifica: %v", err)
	}
	if len(full.Clients) != 1 || len(full.Charges) != 1 || len(full.Orders) != 1 || len(full.Payments) != 1 {
		t.Fatalf("documentos = %d clientes, %d cargos, %d pedidos, %d pagos; se esperaba 1 de cada uno",
			len(full.Clients), len(full.Charges), len(full.Orders), len(full.Payments))
	}
	if c := full.Clients[0]; c.Age != 40 || c.UUID != "cli-1" || c.CreateAt.IsZero() {
		t.Errorf("cliente = %+v", c)
	}
	if p := full.Charges[0].Products[0]; p.Key != "S1122" || p.Page != "12" || p.Price != 150.5 || p.Quantity != 2 {
		t.Errorf("producto del cargo = %+v", p)
	}
	if p := full.Payments[0]; p.Amount != 150 || p.ClientUUID != "cli-1" {
		t.Errorf("pago = %+v", p)
	}

	tests := []struct {
		name  string
		body  string
		check func(models.RequestUpdate) bool
	}{
		{"teléfono como número", `{"clients":[{"phone":2221234567}]}`,
			func(r models.RequestUpdate) bool { return r.Clients[0].Phone == float64(2221234567) }},
		{"teléfono como texto", `{"clients":[{"phone":"222 123 4567"}]}`,
			func(r models.RequestUpdate) bool { return r.Clients[0].Phone == "222 123 4567" }},
		{"edad ausente queda en 0", `{"clients":[{"uuid":"cli-1"}]}`,
			func(r models.RequestUpdate) bool { return r.Clients[0].Age == 0 }},
		{"edad null queda en 0", `{"clients":[{"age":null}]}`,
			func(r models.RequestUpdate) bool { return r.Clients[0].Age == 0 }},
		{"monto entero", `{"payments":[{"amount":150}]}`,
			func(r models.RequestUpdate) bool { return r.Payments[0].Amount == 150 }},
		{"fecha con zona horaria", `{"payments":[{"createat":"2026-10-20T09:07:00-06:00"}]}`,
			func(r models.RequestUpdate) bool { return r.Payments[0].CreateAt.UTC().Hour() == 15 }},
		{"tipo de cliente y campos cambiados", `{"clients":[{"type":"blacklist","changedfields":["type","lastname"]}]}`,
			func(r models.RequestUpdate) bool {
				c := r.Clients[0]
				return c.Type == "blacklist" && len(c.ChangedFields) == 2 && c.ChangedFields[1] == "lastname"
			}},
		{"sin tipo ni campos cambiados (versión anterior de la app)", `{"clients":[{"uuid":"cli-1"}]}`,
			func(r models.RequestUpdate) bool { return r.Clients[0].Type == "" && r.Clients[0].ChangedFields == nil }},
		{"campos desconocidos se ignoran", `{"clients":[{"uuid":"cli-1","color":"rojo"}],"extra":true}`,
			func(r models.RequestUpdate) bool { return r.Clients[0].UUID == "cli-1" }},
		{"colecciones ausentes", `{}`,
			func(r models.RequestUpdate) bool { return r.Clients == nil && r.Payments == nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got models.RequestUpdate
			if err := json.Unmarshal([]byte(tt.body), &got); err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !tt.check(got) {
				t.Errorf("decodificado = %+v", got)
			}
		})
	}
}
