package handlers

import (
	"context"
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Topes con MongoDB (se saltan sin CLOUSESTONE_TEST_MONGO_URI, ver mongotest):
//   - el login deja a lo más 5 sesiones vigentes; una vencida o quitada libera lugar;
//   - el respaldo rechaza sólo los clientes nuevos que no caben en el tope de la cuenta.

// loginRecorder entra como la app y regresa la respuesta tal cual (serveLogin exige 200).
func loginRecorder(t *testing.T, db *database.MongoClient, email, device string) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("ACCESS_SECRET", "secreto-de-acceso-de-prueba")
	t.Setenv("REFRESH_SECRET", "secreto-de-renovacion-de-prueba")
	body := fmt.Sprintf(`{"email":%q,"password":%q,"device":%q}`, email, testPassword, device)
	rec := httptest.NewRecorder()
	Login(db).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body)))
	return rec
}

// getJwtRecorder renueva el token como serveGetJwt, pero regresa la respuesta tal cual.
func getJwtRecorder(t *testing.T, db *database.MongoClient, userID primitive.ObjectID, device string, login map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	ctx := context.WithValue(context.Background(), ctxkeys.UserID, userID.Hex())
	ctx = context.WithValue(ctx, ctxkeys.DeviceID, device)
	ctx = context.WithValue(ctx, ctxkeys.Cookie, login["refreshtoken"])
	rec := httptest.NewRecorder()
	RefreshToken(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/GetJwt", nil).WithContext(ctx))
	return rec
}

// expireSession le pone a la sesión del dispositivo la fecha de vencimiento at.
func expireSession(t *testing.T, db *database.MongoClient, userID primitive.ObjectID, device string, at time.Time) {
	t.Helper()
	_, err := mongotest.Raw(t, db).Collection("devices").UpdateOne(context.Background(),
		bson.D{{Key: "_id", Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "devices.$[d].refreshtoken.dateend", Value: at}}}},
		options.Update().SetArrayFilters(options.ArrayFilters{Filters: []interface{}{bson.M{"d.uuid": device}}}))
	if err != nil {
		t.Fatalf("vencer la sesión de %s: %v", device, err)
	}
}

func deviceUUIDs(t *testing.T, db *database.MongoClient, userID primitive.ObjectID) []string {
	t.Helper()
	doc, err := db.FindDevice(context.Background(), bson.D{{Key: "_id", Value: userID}})
	if err != nil {
		t.Fatalf("FindDevice: %v", err)
	}
	var uuids []string
	for _, d := range doc.Devices {
		uuids = append(uuids, d.UUID)
	}
	return sorted(uuids)
}

func TestLoginDeviceLimitWithMongo(t *testing.T) {
	db := mongotest.Connect(t)
	const email = "dispositivos@ejemplo.mx"
	user := registerWithPassword(t, db, email)

	logins := map[string]map[string]any{}
	for i := 1; i <= 5; i++ {
		device := fmt.Sprintf("disp-%d", i)
		logins[device] = serveLogin(t, db, email, device)
	}

	// El sexto no entra y no queda registrado
	rec := loginRecorder(t, db, email, "disp-6")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sexto dispositivo: código = %d; se esperaba 403 (%s)", rec.Code, rec.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "device_limit" {
		t.Errorf("sexto dispositivo: cuerpo = %s", rec.Body)
	}
	want := []string{"disp-1", "disp-2", "disp-3", "disp-4", "disp-5"}
	if got := deviceUUIDs(t, db, user.ID); !reflect.DeepEqual(got, want) {
		t.Errorf("dispositivos = %v; se esperaban %v", got, want)
	}

	// Uno de los cinco siempre puede volver a entrar
	serveLogin(t, db, email, "disp-3")

	// Una sesión vencida ya no cuenta: el sexto entra
	expireSession(t, db, user.ID, "disp-1", time.Now().Add(-time.Hour))
	serveLogin(t, db, email, "disp-6")

	// Cerrar una sesión desde la base (como la consola) la saca y libera lugar
	if err := db.RemoveDeviceByUUID(context.Background(), user.ID, "disp-2"); err != nil {
		t.Fatalf("RemoveDeviceByUUID: %v", err)
	}
	if rec := getJwtRecorder(t, db, user.ID, "disp-2", logins["disp-2"]); rec.Code != http.StatusUnauthorized {
		t.Errorf("/GetJwt del dispositivo cerrado: código = %d; se esperaba 401", rec.Code)
	}
	serveLogin(t, db, email, "disp-7")
	if rec := loginRecorder(t, db, email, "disp-8"); rec.Code != http.StatusForbidden {
		t.Errorf("con 5 sesiones vigentes otra vez, disp-8: código = %d; se esperaba 403", rec.Code)
	}
}

func TestUploadDataClientLimitWithMongo(t *testing.T) {
	db := mongotest.Connect(t)
	const email = "tope-respaldo@ejemplo.mx"
	user := registerPaidUser(t, db, email)
	setClientLimit(t, db, email, int32(3))
	stored := func(collection string) []string {
		var uuids []string
		for _, doc := range storedDocs(t, db, user.ID)[collection] {
			uuids = append(uuids, doc.uuid)
		}
		return sorted(uuids)
	}

	if rec := serveUpload(t, db, email, testDeviceID, backupBody([]string{clientRecord("c1", "active", 1)})); rec.Code != http.StatusOK {
		t.Fatalf("primer cliente: código = %d (%s)", rec.Code, rec.Body)
	}

	// Quedan 2 lugares: entran los dos registrados primero; el pago siempre pasa
	rec := serveUpload(t, db, email, testDeviceID, backupBody([]string{
		clientRecord("c5", "active", 5), clientRecord("c2", "active", 2),
		clientRecord("c4", "active", 4), clientRecord("c3", "active", 3),
	}, limitPayment))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("código = %d; se esperaba 422 (%s)", rec.Code, rec.Body)
	}
	var rejected []string
	for _, r := range rejectedOf(t, rec.Body.Bytes()) {
		if r.Code != "client_limit" {
			t.Errorf("rechazo sin client_limit: %+v", r)
		}
		rejected = append(rejected, r.UUID)
	}
	if got := sorted(rejected); !reflect.DeepEqual(got, []string{"c4", "c5"}) {
		t.Errorf("rechazados = %v; se esperaban [c4 c5]", got)
	}
	if got := stored("clients"); !reflect.DeepEqual(got, []string{"c1", "c2", "c3"}) {
		t.Errorf("clientes en la nube = %v", got)
	}
	if got := stored("payments"); !reflect.DeepEqual(got, []string{"pag-1"}) {
		t.Errorf("pagos en la nube = %v", got)
	}

	// En el tope, un cliente que ya está se sigue actualizando
	if rec := serveUpload(t, db, email, testDeviceID, backupBody([]string{clientRecord("c2", "active", 2)})); rec.Code != http.StatusOK {
		t.Errorf("actualizar un cliente existente en el tope: código = %d (%s)", rec.Code, rec.Body)
	}

	// Al ampliarle el tope, el siguiente respaldo sube los que faltaban
	setClientLimit(t, db, email, int32(5))
	if rec := serveUpload(t, db, email, testDeviceID, backupBody([]string{clientRecord("c4", "active", 4), clientRecord("c5", "active", 5)})); rec.Code != http.StatusOK {
		t.Fatalf("con el tope ampliado: código = %d (%s)", rec.Code, rec.Body)
	}
	if got := stored("clients"); !reflect.DeepEqual(got, []string{"c1", "c2", "c3", "c4", "c5"}) {
		t.Errorf("clientes en la nube con el tope ampliado = %v", got)
	}
}

// Las dos consultas nuevas del respaldo, contra Mongo.
func TestExistingUUIDsAndCountActiveClients(t *testing.T) {
	db := mongotest.Connect(t)
	user := registerPaidUser(t, db, "consultas-tope@ejemplo.mx")
	other := registerPaidUser(t, db, "otra-cuenta@ejemplo.mx")
	body := backupBody([]string{clientRecord("a", "active", 1), clientRecord("b", "deleted", 2), clientRecord("c", "active", 3)})
	if rec := serveUpload(t, db, "consultas-tope@ejemplo.mx", testDeviceID, body); rec.Code != http.StatusOK {
		t.Fatalf("código = %d (%s)", rec.Code, rec.Body)
	}
	ctx := context.Background()
	found, err := db.ExistingUUIDs(ctx, "clients", user.ID.Hex(), []string{"a", "b", "z"})
	if err != nil || !reflect.DeepEqual(found, map[string]bool{"a": true, "b": true}) {
		t.Errorf("ExistingUUIDs = %v, %v; se esperaban a y b", found, err)
	}
	if found, _ := db.ExistingUUIDs(ctx, "clients", other.ID.Hex(), []string{"a"}); len(found) != 0 {
		t.Errorf("ExistingUUIDs de otra cuenta = %v; se esperaba vacío", found)
	}
	if n, err := db.CountActiveClients(ctx, user.ID.Hex()); err != nil || n != 2 {
		t.Errorf("CountActiveClients = %d, %v; se esperaban 2 (el borrado no cuenta)", n, err)
	}
}
