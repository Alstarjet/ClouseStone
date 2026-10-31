package handlers

import (
	"context"
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"financial-Assistant/internal/mainservice/models"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Estas pruebas necesitan MongoDB: se saltan si no está CLOUSESTONE_TEST_MONGO_URI
// (ver mongotest).

func registerUser(t *testing.T, db *database.MongoClient, email string) models.User {
	t.Helper()
	user := models.User{Name: "Ana", LastName: "López", Email: email}
	if _, err := db.RegisterUser(&user); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	return user
}

// storedDoc es lo que se revisa de cada documento guardado.
type storedDoc struct {
	id     primitive.ObjectID
	uuid   string
	detail any // nombre del cliente o monto del pago; nil en cargos y pedidos
}

// storedDocs lee lo que el usuario tiene guardado en cada colección del respaldo.
func storedDocs(t *testing.T, db *database.MongoClient, userID primitive.ObjectID) map[string][]storedDoc {
	t.Helper()
	filter := bson.D{{Key: "usermongoid", Value: userID.Hex()}}
	docs := map[string][]storedDoc{}

	clients, err := db.FindAllClients(filter)
	if err != nil {
		t.Fatalf("FindAllClients: %v", err)
	}
	for _, c := range clients {
		docs["clients"] = append(docs["clients"], storedDoc{c.ID, c.UUID, c.Name})
	}
	charges, err := db.FindAllCharges(filter)
	if err != nil {
		t.Fatalf("FindAllCharges: %v", err)
	}
	for _, c := range charges {
		docs["charges"] = append(docs["charges"], storedDoc{c.ID, c.UUID, nil})
	}
	orders, err := db.FindAllOrders(filter)
	if err != nil {
		t.Fatalf("FindAllOrders: %v", err)
	}
	for _, o := range orders {
		docs["orders"] = append(docs["orders"], storedDoc{o.ID, o.UUID, nil})
	}
	payments, err := db.FindAllPayments(filter)
	if err != nil {
		t.Fatalf("FindAllPayments: %v", err)
	}
	for _, p := range payments {
		docs["payments"] = append(docs["payments"], storedDoc{p.ID, p.UUID, p.Amount})
	}
	return docs
}

var backupUUIDs = map[string]string{"clients": "cli-1", "charges": "car-1", "orders": "ped-1", "payments": "pag-1"}

func TestUploadDataBackup(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerUser(t, db, testEmail)

	rec := serveUpload(t, db, testEmail, testDeviceID, validBackup)
	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d; se esperaba 200 (cuerpo: %s)", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("respuesta no es JSON: %v (%s)", err, rec.Body)
	}
	// ⚠️ El estado viaja con la clave "reports", no "status" (models.BackupResponse;
	// documentado en endpoints/data-uploaddata.md). La PWA sólo lee el código HTTP.
	want := map[string]any{"message": "Datos Respaldados con Éxito", "reports": float64(200), "typeclient": "Quartz"}
	for k, v := range want {
		if resp[k] != v {
			t.Errorf("respuesta[%q] = %v; se esperaba %v", k, resp[k], v)
		}
	}
	if len(resp) != len(want) {
		t.Errorf("respuesta = %v; se esperaban sólo las claves %v", resp, want)
	}

	docs := storedDocs(t, db, user.ID)
	for coll, uuid := range backupUUIDs {
		t.Run(coll, func(t *testing.T) {
			if got := docs[coll]; len(got) != 1 || got[0].uuid != uuid {
				t.Errorf("guardados = %+v; se esperaba sólo %q", got, uuid)
			}
		})
	}

	// Los tipos llegan intactos a Mongo.
	clients, err := db.FindAllClients(bson.D{{Key: "uuid", Value: "cli-1"}})
	if err != nil || len(clients) != 1 {
		t.Fatalf("cliente guardado = %v, %v", clients, err)
	}
	if c := clients[0]; c.Age != 40 || c.Phone != "2221234567" || c.UserMongoID != user.ID.Hex() || c.BackupAt.IsZero() {
		t.Errorf("cliente guardado = %+v", c)
	}
}

// Subir otra vez los mismos uuid actualiza los documentos en lugar de duplicarlos;
// el uuid sólo identifica dentro de cada usuario.
func TestUploadDataUpsertsByUUID(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerUser(t, db, testEmail)
	other := registerUser(t, db, "luis@ejemplo.mx")

	if rec := serveUpload(t, db, testEmail, testDeviceID, validBackup); rec.Code != http.StatusOK {
		t.Fatalf("primer respaldo: código = %d (%s)", rec.Code, rec.Body)
	}
	first := storedDocs(t, db, user.ID)

	edited := strings.NewReplacer(`"name":"Ana"`, `"name":"Ana María"`, `"amount":150`, `"amount":175.5`).Replace(validBackup)
	if rec := serveUpload(t, db, testEmail, testDeviceID, edited); rec.Code != http.StatusOK {
		t.Fatalf("segundo respaldo: código = %d (%s)", rec.Code, rec.Body)
	}
	if rec := serveUpload(t, db, "luis@ejemplo.mx", "dispositivo-luis", validBackup); rec.Code != http.StatusOK {
		t.Fatalf("respaldo de otro usuario: código = %d (%s)", rec.Code, rec.Body)
	}

	second := storedDocs(t, db, user.ID)
	wantDetail := map[string]any{"clients": "Ana María", "payments": 175.5}
	for coll, uuid := range backupUUIDs {
		t.Run(coll, func(t *testing.T) {
			got := second[coll]
			if len(got) != 1 || got[0].uuid != uuid {
				t.Fatalf("guardados = %+v; se esperaba un solo %q", got, uuid)
			}
			if got[0].id != first[coll][0].id {
				t.Errorf("_id cambió: %s → %s", first[coll][0].id.Hex(), got[0].id.Hex())
			}
			if want, ok := wantDetail[coll]; ok && got[0].detail != want {
				t.Errorf("no se actualizó: %v; se esperaba %v", got[0].detail, want)
			}
		})
	}

	// El mismo respaldo de otro usuario crea sus propios documentos.
	others := storedDocs(t, db, other.ID)
	for coll := range backupUUIDs {
		if got := others[coll]; len(got) != 1 || got[0].id == first[coll][0].id {
			t.Errorf("%s de otro usuario = %+v; se esperaba un documento propio", coll, got)
		}
	}
}

// clientUpload arma un respaldo con un solo cliente: la copia completa que
// tiene el dispositivo, con su tipo y los campos que cambió (si los hay).
func clientUpload(t *testing.T, name, lastname, clientType string, changed ...string) string {
	t.Helper()
	client := map[string]any{
		"uuid": "cli-1", "name": name, "lastname": lastname, "age": 40, "city": "Puebla",
		"neighborhood": "Centro", "address": "5 de Mayo 10", "phone": "2221234567", "daywork": "lunes",
		"createat": "2026-10-20T15:04:05Z", "updateat": "2026-10-30T15:04:05Z", "status": "active",
	}
	if clientType != "" {
		client["type"] = clientType
	}
	if changed != nil {
		client["changedfields"] = changed
	}
	body, err := json.Marshal(map[string]any{"clients": []any{client}, "deviceid": testDeviceID})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(body)
}

func mustUpload(t *testing.T, db *database.MongoClient, email, body string) {
	t.Helper()
	if rec := serveUpload(t, db, email, testDeviceID, body); rec.Code != http.StatusOK {
		t.Fatalf("respaldo: código = %d (%s)", rec.Code, rec.Body)
	}
}

// storedClient lee cli-1 del usuario tal como quedó en Mongo (y que sea uno solo).
func storedClient(t *testing.T, db *database.MongoClient, userID primitive.ObjectID) models.Client {
	t.Helper()
	clients, err := db.FindAllClients(bson.D{{Key: "usermongoid", Value: userID.Hex()}})
	if err != nil || len(clients) != 1 {
		t.Fatalf("clientes guardados = %+v, %v; se esperaba uno", clients, err)
	}
	return clients[0]
}

// serveGetDataSince baja lo que llegó desde since, como otro dispositivo.
func serveGetDataSince(t *testing.T, db *database.MongoClient, email string, since time.Time) []map[string]any {
	t.Helper()
	ctx := context.WithValue(context.Background(), ctxkeys.Email, email)
	target := "/GetDataSince?since=" + url.QueryEscape(since.Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	GetDataSince(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetDataSince: código = %d (%s)", rec.Code, rec.Body)
	}
	var resp struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetDataSince: respuesta no es JSON: %v (%s)", err, rec.Body)
	}
	return resp.Clients
}

// Mandar a lista negra sólo cambia "type" y le llega al otro dispositivo en la
// siguiente descarga (backupat se vuelve a sellar), sin changedfields.
func TestUploadDataClientTypeReachesOtherDevices(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerUser(t, db, testEmail)
	mustUpload(t, db, testEmail, clientUpload(t, "Ana", "López", ""))

	since := time.Now().UTC()
	time.Sleep(5 * time.Millisecond)
	mustUpload(t, db, testEmail, clientUpload(t, "Ana", "López", "blacklist", "type"))

	if c := storedClient(t, db, user.ID); c.Type != "blacklist" || c.Lastname != "López" {
		t.Errorf("cliente guardado = %+v", c)
	}
	down := serveGetDataSince(t, db, testEmail, since)
	if len(down) != 1 || down[0]["uuid"] != "cli-1" || down[0]["type"] != "blacklist" {
		t.Fatalf("baja = %v; se esperaba cli-1 con type blacklist", down)
	}
	if _, ok := down[0]["changedfields"]; ok {
		t.Errorf("baja con changedfields: %v", down[0])
	}
}

// El caso de dos dispositivos sin sincronizar: A manda al cliente a lista
// negra y B le corrige el apellido. En cualquier orden se quedan los dos cambios.
func TestUploadDataChangedFieldsKeepOtherDevices(t *testing.T) {
	db := mongotest.Connect(t)
	fromA := clientUpload(t, "Ana", "Lopez", "blacklist", "type")
	fromB := clientUpload(t, "Ana", "López Ruiz", "", "lastname")

	tests := []struct {
		name, email string
		uploads     []string
	}{
		{"A y luego B", "orden-ab@ejemplo.mx", []string{fromA, fromB}},
		{"B y luego A", "orden-ba@ejemplo.mx", []string{fromB, fromA}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := registerUser(t, db, tt.email)
			mustUpload(t, db, tt.email, clientUpload(t, "Ana", "Lopez", ""))
			first := storedClient(t, db, user.ID)
			for _, body := range tt.uploads {
				mustUpload(t, db, tt.email, body)
			}
			c := storedClient(t, db, user.ID)
			if c.Type != "blacklist" || c.Lastname != "López Ruiz" || c.Name != "Ana" || c.City != "Puebla" {
				t.Errorf("cliente guardado = %+v; se esperaba lista negra y el apellido corregido", c)
			}
			if c.ID != first.ID {
				t.Errorf("_id cambió: %s → %s", first.ID.Hex(), c.ID.Hex())
			}
		})
	}
}

func TestUploadDataClientTypeChanges(t *testing.T) {
	blacklisted := clientUpload(t, "Ana", "López", "blacklist")
	tests := []struct {
		name     string
		uploads  []string
		wantName string
		wantType string
	}{
		// Una versión de la app que no conoce el campo manda el documento
		// completo sin type: actualiza lo demás y no saca al cliente de la lista.
		{"respaldo sin changedfields no borra la lista negra",
			[]string{blacklisted, clientUpload(t, "Ana María", "López", "")}, "Ana María", "blacklist"},
		{"sacar de la lista negra quita el campo",
			[]string{blacklisted, clientUpload(t, "Ana", "López", "", "type")}, "Ana", ""},
		{"un cliente nuevo con changedfields se guarda completo",
			[]string{clientUpload(t, "Ana", "López", "", "lastname")}, "Ana", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := mongotest.Connect(t)
			user := registerUser(t, db, testEmail)
			for _, body := range tt.uploads {
				mustUpload(t, db, testEmail, body)
			}
			c := storedClient(t, db, user.ID)
			if c.Name != tt.wantName || c.Type != tt.wantType || c.City != "Puebla" {
				t.Errorf("cliente guardado = %+v", c)
			}
			withType, err := db.FindAllClients(bson.D{
				{Key: "usermongoid", Value: user.ID.Hex()},
				{Key: "type", Value: bson.M{"$exists": true}},
			})
			if err != nil {
				t.Fatalf("FindAllClients: %v", err)
			}
			if stored, want := len(withType) == 1, tt.wantType != ""; stored != want {
				t.Errorf("¿type guardado en Mongo? = %v; se esperaba %v", stored, want)
			}
		})
	}
}
