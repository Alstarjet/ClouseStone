package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/devices"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Estas pruebas necesitan MongoDB: se saltan si no está CLOUSESTONE_TEST_MONGO_URI
// (ver mongotest). Las cuentas que respaldan son del plan de pago (registerPaidUser):
// la versión gratis no respalda (plans_mongo_test.go).

func registerUser(t *testing.T, db *database.MongoClient, email string) models.User {
	t.Helper()
	user := models.User{Name: "Ana", LastName: "López", Email: email}
	if _, err := db.RegisterUser(context.Background(), &user); err != nil {
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
	ctx := context.Background()
	filter := bson.D{{Key: "usermongoid", Value: userID.Hex()}}
	docs := map[string][]storedDoc{}

	clients, err := db.FindAllClients(ctx, filter)
	if err != nil {
		t.Fatalf("FindAllClients: %v", err)
	}
	for _, c := range clients {
		docs["clients"] = append(docs["clients"], storedDoc{c.ID, c.UUID, string(c.Name)})
	}
	charges, err := db.FindAllCharges(ctx, filter)
	if err != nil {
		t.Fatalf("FindAllCharges: %v", err)
	}
	for _, c := range charges {
		docs["charges"] = append(docs["charges"], storedDoc{c.ID, c.UUID, nil})
	}
	orders, err := db.FindAllOrders(ctx, filter)
	if err != nil {
		t.Fatalf("FindAllOrders: %v", err)
	}
	for _, o := range orders {
		docs["orders"] = append(docs["orders"], storedDoc{o.ID, o.UUID, nil})
	}
	payments, err := db.FindAllPayments(ctx, filter)
	if err != nil {
		t.Fatalf("FindAllPayments: %v", err)
	}
	for _, p := range payments {
		docs["payments"] = append(docs["payments"], storedDoc{p.ID, p.UUID, p.Amount})
	}
	return docs
}

var backupUUIDs = goodUUIDs

func TestUploadDataBackup(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerPaidUser(t, db, testEmail)

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
	want := map[string]any{"message": "Datos Respaldados con Éxito", "reports": float64(200), "typeclient": "Amethyst"}
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

	// Los tipos llegan intactos a Mongo, con su dueño y la hora del servidor.
	clients, err := db.FindAllClients(context.Background(), bson.D{{Key: "uuid", Value: "cli-1"}})
	if err != nil || len(clients) != 1 {
		t.Fatalf("cliente guardado = %v, %v", clients, err)
	}
	if c := clients[0]; c.Age != 40 || c.Phone != "2221234567" || c.UserMongoID != user.ID.Hex() || c.BackupAt.IsZero() {
		t.Errorf("cliente guardado = %+v", c)
	}
	charges, err := db.FindAllCharges(context.Background(), bson.D{{Key: "uuid", Value: "car-1"}})
	if err != nil || len(charges) != 1 || charges[0].CreateAt.IsZero() || len(charges[0].Products) != 1 {
		t.Fatalf("cargo guardado = %+v, %v", charges, err)
	}
	if p := charges[0].Products[0]; p.UUID != "prod-1" || p.Quantity != 2 || p.Total != 301 {
		t.Errorf("renglón guardado = %+v; se esperaba con su uuid (I-12)", p)
	}

	// I-07: el respaldo ya no crea ni toca el documento de dispositivos
	if _, err := db.FindDevice(context.Background(), bson.D{{Key: "_id", Value: user.ID}}); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("FindDevice = %v; el respaldo no debe crear el documento de dispositivos", err)
	}
}

// Subir otra vez los mismos uuid actualiza los documentos en lugar de duplicarlos;
// el uuid sólo identifica dentro de cada usuario.
func TestUploadDataUpsertsByUUID(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerPaidUser(t, db, testEmail)
	other := registerPaidUser(t, db, "luis@ejemplo.mx")

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

// I-02 con Mongo: con un registro malo, lo demás del lote queda guardado y el 422
// dice cuál no.
func TestUploadDataSavesAllButRejected(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerPaidUser(t, db, testEmail)

	rec := serveUpload(t, db, testEmail, testDeviceID, withBad("clients", `{"uuid":"cli-2","name":"Luis","age":"40"}`))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("código = %d; se esperaba 422 (cuerpo: %s)", rec.Code, rec.Body)
	}
	var resp models.BackupRejectedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || len(resp.Rejected) != 1 || resp.Rejected[0].UUID != "cli-2" {
		t.Fatalf("respuesta = %+v, %v", resp, err)
	}
	docs := storedDocs(t, db, user.ID)
	for coll, uuid := range backupUUIDs {
		if got := docs[coll]; len(got) != 1 || got[0].uuid != uuid {
			t.Errorf("%s guardados = %+v; se esperaba sólo %q", coll, got, uuid)
		}
	}
}

// I-08: dos (o más) subidas a la vez del mismo lote, como dos pestañas o un
// reintento, dejan un solo documento por uuid. Necesita el índice único, como en
// producción al arrancar (EnsureSyncIndexes).
func TestUploadDataConcurrentUploadsDoNotDuplicate(t *testing.T) {
	db := mongotest.Connect(t)
	if err := db.EnsureSyncIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureSyncIndexes: %v", err)
	}
	user := registerPaidUser(t, db, testEmail)

	const uploads = 8
	codes := make(chan int, uploads)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < uploads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rec := httptest.NewRecorder()
			UploadDataSchedule(db).ServeHTTP(rec, uploadRequest(testEmail, testDeviceID, validBackup))
			codes <- rec.Code
		}()
	}
	close(start)
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Errorf("código = %d; se esperaba 200 (Mongo reintenta solo el upsert que choca con el índice)", code)
		}
	}
	docs := storedDocs(t, db, user.ID)
	for coll, uuid := range backupUUIDs {
		if got := docs[coll]; len(got) != 1 || got[0].uuid != uuid {
			t.Errorf("%s = %+v; se esperaba un solo %q", coll, got, uuid)
		}
	}
}

// devicesDoc lee el documento de dispositivos tal como está en Mongo.
func devicesDoc(t *testing.T, db *database.MongoClient, userID primitive.ObjectID) bson.M {
	t.Helper()
	var doc bson.M
	err := mongotest.Raw(t, db).Collection("devices").FindOne(context.Background(), bson.M{"_id": userID}).Decode(&doc)
	if err != nil {
		t.Fatalf("leer dispositivos: %v", err)
	}
	return doc
}

// I-07: respaldar no toca el documento de dispositivos: ni la cola de los demás
// dispositivos ni sus refresh tokens.
func TestUploadDataDoesNotTouchDevices(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerPaidUser(t, db, testEmail)
	ctx := context.Background()
	for _, device := range []string{testDeviceID, "dispositivo-2"} {
		if err := devices.AddDeviceAndRefreshToken(ctx, db, user, "token-"+device, time.Now().Add(time.Hour), device); err != nil {
			t.Fatalf("AddDeviceAndRefreshToken: %v", err)
		}
	}
	before := devicesDoc(t, db, user.ID)

	for i := 0; i < 3; i++ {
		if rec := serveUpload(t, db, testEmail, testDeviceID, validBackup); rec.Code != http.StatusOK {
			t.Fatalf("respaldo %d: código = %d (%s)", i, rec.Code, rec.Body)
		}
	}
	if after := devicesDoc(t, db, user.ID); !reflect.DeepEqual(before, after) {
		t.Errorf("el respaldo cambió el documento de dispositivos:\nantes   %v\ndespués %v", before, after)
	}
}

// /GetData y /DeleteIds (legado, los llama todavía una pantalla vieja de la app):
// lo que quedó en la cola baja como documentos actuales del usuario, sin vacíos ni
// repetidos, y vaciarla no toca los tokens ni la cola de otro dispositivo.
func TestLegacyQueueIsHarmless(t *testing.T) {
	db := mongotest.Connect(t)
	ctx := context.Background()
	user := registerPaidUser(t, db, testEmail)
	other := registerPaidUser(t, db, "luis@ejemplo.mx")
	mustUpload(t, db, testEmail, validBackup)
	mustUpload(t, db, "luis@ejemplo.mx", strings.Replace(validBackup, `"name":"Ana"`, `"name":"Luis"`, 1))
	mine := storedDocs(t, db, user.ID)["clients"][0].id.Hex()
	theirs := storedDocs(t, db, other.ID)["clients"][0].id.Hex()
	missing := primitive.NewObjectID().Hex()

	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Millisecond)
	queue := func(uuid, token string, clientIDs ...string) bson.M {
		return bson.M{"uuid": uuid, "clientids": clientIDs, "chargeids": bson.A{}, "orderids": bson.A{}, "paymentids": bson.A{},
			"refreshtoken": bson.M{"token": token, "dateend": expires}}
	}
	_, err := mongotest.Raw(t, db).Collection("devices").InsertOne(ctx, bson.M{
		"_id": user.ID, "usermongoid": user.ID.Hex(),
		"devices": bson.A{queue("tel-1", "hash-1", mine, theirs, missing, mine), queue("tel-2", "hash-2", mine)},
	})
	if err != nil {
		t.Fatalf("preparar la cola: %v", err)
	}

	emailCtx := context.WithValue(ctx, ctxkeys.Email, testEmail)
	rec := httptest.NewRecorder()
	GetData(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/GetData?deviceid=tel-1", nil).WithContext(emailCtx))
	if rec.Code != http.StatusOK {
		t.Fatalf("GetData: código = %d (%s)", rec.Code, rec.Body)
	}
	var data models.AllData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if len(data.Clients) != 1 || data.Clients[0].UUID != "cli-1" || data.Clients[0].Name != "Ana" {
		t.Errorf("GetData clientes = %+v; se esperaba sólo cli-1 del usuario (Ana), una vez", data.Clients)
	}

	rec = httptest.NewRecorder()
	DeleteDocIds(db).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/DeleteIds?deviceid=tel-1", nil).WithContext(emailCtx))
	if rec.Code != http.StatusOK {
		t.Fatalf("DeleteIds: código = %d (%s)", rec.Code, rec.Body)
	}
	doc, err := db.FindDevice(ctx, bson.D{{Key: "_id", Value: user.ID}})
	if err != nil || len(doc.Devices) != 2 {
		t.Fatalf("dispositivos = %+v, %v", doc, err)
	}
	for _, d := range doc.Devices {
		switch d.UUID {
		case "tel-1":
			if len(d.ClientIDs) != 0 || d.Refreshtoken.Token != "hash-1" {
				t.Errorf("tel-1 = %+v; se esperaba la cola vacía y su token", d)
			}
		case "tel-2":
			if !reflect.DeepEqual(d.ClientIDs, []string{mine}) || d.Refreshtoken.Token != "hash-2" {
				t.Errorf("tel-2 = %+v; no se debía tocar", d)
			}
		}
	}
}

// I-09: si no se pueden leer todos los datos de la cuenta, el login responde 500
// (antes: 200 con la cuenta incompleta) y no registra el dispositivo.
func TestLoginFailsWhenDataCannotBeRead(t *testing.T) {
	db := mongotest.Connect(t)
	const email = "datos-rotos@ejemplo.mx"
	user := registerWithPassword(t, db, email)
	setPlan(t, db, email, "Amethyst", nil)
	// Un pedido que no se puede leer como models.Charge (createat no es fecha)
	_, err := mongotest.Raw(t, db).Collection("orders").InsertOne(context.Background(),
		bson.M{"usermongoid": user.ID.Hex(), "uuid": "ped-roto", "createat": "no es fecha"})
	if err != nil {
		t.Fatalf("preparar el pedido: %v", err)
	}

	t.Setenv("ACCESS_SECRET", "secreto-de-acceso-de-prueba")
	t.Setenv("REFRESH_SECRET", "secreto-de-renovacion-de-prueba")
	body := fmt.Sprintf(`{"email":%q,"password":%q,"device":"dispositivo-roto"}`, email, testPassword)
	rec := httptest.NewRecorder()
	Login(db).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body)))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("código = %d; se esperaba 500 (cuerpo: %s)", rec.Code, rec.Body)
	}
	if _, err := db.FindDevice(context.Background(), bson.D{{Key: "_id", Value: user.ID}}); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("FindDevice = %v; un login fallido no registra el dispositivo", err)
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
	clients, err := db.FindAllClients(context.Background(), bson.D{{Key: "usermongoid", Value: userID.Hex()}})
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
	user := registerPaidUser(t, db, testEmail)
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
			user := registerPaidUser(t, db, tt.email)
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
			user := registerPaidUser(t, db, testEmail)
			for _, body := range tt.uploads {
				mustUpload(t, db, testEmail, body)
			}
			c := storedClient(t, db, user.ID)
			if string(c.Name) != tt.wantName || c.Type != tt.wantType || c.City != "Puebla" {
				t.Errorf("cliente guardado = %+v", c)
			}
			withType, err := db.FindAllClients(context.Background(), bson.D{
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
