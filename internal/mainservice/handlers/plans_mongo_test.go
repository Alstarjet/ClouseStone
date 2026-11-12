package handlers

import (
	"context"
	"encoding/json"
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/database/mongotest"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

// Planes con MongoDB (se saltan sin CLOUSESTONE_TEST_MONGO_URI, ver mongotest):
//   - la versión gratis, o un plan desconocido, no respalda: 403 y nada escrito;
//   - el plan de pago guarda todo lo que manda, aunque pase de su tope o ya haya vencido;
//   - el login y /GetJwt regresan las reglas del plan y su vigencia;
//   - registrarse siempre deja la versión gratis, sin fecha.

const testPassword = "clave-de-prueba-1"

// setPlan cambia el plan (y su vigencia, si se da) de la cuenta como se hace a mano en la base.
func setPlan(t *testing.T, db *database.MongoClient, email, typeclient string, expires *time.Time) {
	t.Helper()
	set := bson.M{"typeclient": typeclient}
	if expires != nil {
		set["planexpiresat"] = *expires
	}
	result, err := mongotest.Raw(t, db).Collection("users").UpdateOne(context.Background(),
		bson.M{"email": email}, bson.M{"$set": set})
	if err != nil {
		t.Fatalf("setPlan(%q): %v", email, err)
	}
	if result.MatchedCount != 1 {
		t.Fatalf("setPlan(%q): %d cuentas; se esperaba una", email, result.MatchedCount)
	}
}

// setClientLimit le pone a la cuenta su propio tope de clientes (users.clientlimit),
// como se hace a mano en la base; nil se lo quita.
func setClientLimit(t *testing.T, db *database.MongoClient, email string, value any) {
	t.Helper()
	update := bson.M{"$set": bson.M{"clientlimit": value}}
	if value == nil {
		update = bson.M{"$unset": bson.M{"clientlimit": ""}}
	}
	result, err := mongotest.Raw(t, db).Collection("users").UpdateOne(context.Background(), bson.M{"email": email}, update)
	if err != nil {
		t.Fatalf("setClientLimit(%q): %v", email, err)
	}
	if result.MatchedCount != 1 {
		t.Fatalf("setClientLimit(%q): %d cuentas; se esperaba una", email, result.MatchedCount)
	}
}

// registerPaidUser registra la cuenta y le pone el plan de pago, como se hace a mano.
func registerPaidUser(t *testing.T, db *database.MongoClient, email string) models.User {
	t.Helper()
	user := registerUser(t, db, email)
	setPlan(t, db, email, plans.Paid, nil)
	user.TypeClient = plans.Paid
	return user
}

func TestUploadDataWithoutCloudBackupIsRejected(t *testing.T) {
	db := mongotest.Connect(t)
	tests := []struct {
		name, email, typeclient string
	}{
		{"versión gratis", "gratis@ejemplo.mx", plans.Free},
		{"plan desconocido", "raro@ejemplo.mx", "Diamond"},
		{"sin plan", "vacio@ejemplo.mx", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := registerUser(t, db, tt.email)
			setPlan(t, db, tt.email, tt.typeclient, nil)

			rec := serveUpload(t, db, tt.email, testDeviceID, validBackup)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("código = %d; se esperaba 403 (cuerpo: %s)", rec.Code, rec.Body)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"plan_without_backup"}` {
				t.Errorf("cuerpo = %s", got)
			}
			if docs := storedDocs(t, db, user.ID); len(docs) != 0 {
				t.Errorf("se guardó algo: %+v", docs)
			}
			if _, err := db.FindDevice(context.Background(), bson.D{{Key: "_id", Value: user.ID}}); err == nil {
				t.Error("se anotó el dispositivo de una cuenta que no respalda")
			}
		})
	}
}

// El tope de clientes lo cuida el frontend y la vigencia se cambia a mano: el
// backend guarda todo lo que manda una cuenta de pago.
func TestUploadDataPaidPlanSavesEverything(t *testing.T) {
	db := mongotest.Connect(t)

	t.Run("aunque pase de su tope de clientes", func(t *testing.T) {
		const email = "tope@ejemplo.mx"
		user := registerPaidUser(t, db, email)
		limit := plans.For(plans.Paid).ClientLimit
		clients := make([]map[string]any, 0, limit+1)
		for i := 0; i <= limit; i++ {
			clients = append(clients, map[string]any{
				"uuid": fmt.Sprintf("cli-%03d", i), "name": "Cliente", "lastname": fmt.Sprint(i), "age": 30,
				"createat": "2026-11-10T10:00:00Z", "updateat": "2026-11-10T10:00:00Z", "status": "active",
			})
		}
		body, err := json.Marshal(map[string]any{"clients": clients, "deviceid": testDeviceID})
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if rec := serveUpload(t, db, email, testDeviceID, string(body)); rec.Code != http.StatusOK {
			t.Fatalf("código = %d; se esperaba 200 (cuerpo: %s)", rec.Code, rec.Body)
		}
		if got := len(storedDocs(t, db, user.ID)["clients"]); got != limit+1 {
			t.Errorf("clientes guardados = %d; se esperaban %d", got, limit+1)
		}
	})

	t.Run("aunque su vigencia ya haya pasado", func(t *testing.T) {
		const email = "vencido@ejemplo.mx"
		user := registerPaidUser(t, db, email)
		expired := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)
		setPlan(t, db, email, plans.Paid, &expired)
		if rec := serveUpload(t, db, email, testDeviceID, validBackup); rec.Code != http.StatusOK {
			t.Fatalf("código = %d; se esperaba 200 (cuerpo: %s)", rec.Code, rec.Body)
		}
		if docs := storedDocs(t, db, user.ID); len(docs["payments"]) != 1 {
			t.Errorf("guardados = %+v; se esperaba el pago", docs)
		}
	})
}

// registerWithPassword registra una cuenta que puede entrar con testPassword.
func registerWithPassword(t *testing.T, db *database.MongoClient, email string) models.User {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	user := models.User{Name: "Ana", LastName: "López", Email: email, Password: string(hashed)}
	if _, err := db.RegisterUser(context.Background(), &user); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}
	return user
}

// decodeResponse lee la respuesta JSON de un 200.
func decodeResponse(t *testing.T, where string, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: código = %d; se esperaba 200 (cuerpo: %s)", where, rec.Code, rec.Body)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%s: respuesta no es JSON: %v (%s)", where, err, rec.Body)
	}
	return resp
}

// serveLogin entra como lo hace la app (POST /login).
func serveLogin(t *testing.T, db *database.MongoClient, email, device string) map[string]any {
	t.Helper()
	t.Setenv("ACCESS_SECRET", "secreto-de-acceso-de-prueba")
	t.Setenv("REFRESH_SECRET", "secreto-de-renovacion-de-prueba")
	body := fmt.Sprintf(`{"email":%q,"password":%q,"device":%q}`, email, testPassword, device)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	Login(db).ServeHTTP(rec, req)
	return decodeResponse(t, "login", rec)
}

// serveGetJwt renueva el token con el refresh token del login, con lo que deja
// refreshMiddleware en el contexto.
func serveGetJwt(t *testing.T, db *database.MongoClient, user models.User, device string, login map[string]any) map[string]any {
	t.Helper()
	refreshToken, ok := login["refreshtoken"].(string)
	if !ok || refreshToken == "" {
		t.Fatalf("el login no regresó refreshtoken: %v", login)
	}
	ctx := context.WithValue(context.Background(), ctxkeys.UserID, user.ID.Hex())
	ctx = context.WithValue(ctx, ctxkeys.DeviceID, device)
	ctx = context.WithValue(ctx, ctxkeys.Cookie, refreshToken)
	req := httptest.NewRequest(http.MethodGet, "/GetJwt", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	RefreshToken(db).ServeHTTP(rec, req)
	return decodeResponse(t, "/GetJwt", rec)
}

// checkPlan revisa lo del plan en la respuesta del login o de /GetJwt.
func checkPlan(t *testing.T, where string, resp map[string]any, typeclient string, backup bool, limit float64, expires *time.Time) {
	t.Helper()
	if resp["typeclient"] != typeclient || resp["cloudbackup"] != backup || resp["clientlimit"] != limit {
		t.Errorf("%s: typeclient = %v, cloudbackup = %v, clientlimit = %v; se esperaba %q, %v, %v",
			where, resp["typeclient"], resp["cloudbackup"], resp["clientlimit"], typeclient, backup, limit)
	}
	value, ok := resp["planexpiresat"]
	if !ok {
		t.Fatalf("%s: falta planexpiresat", where)
	}
	if expires == nil {
		if value != nil {
			t.Errorf("%s: planexpiresat = %v; se esperaba null (sin fecha)", where, value)
		}
		return
	}
	got, err := time.Parse(time.RFC3339Nano, fmt.Sprint(value))
	if err != nil || !got.Equal(*expires) {
		t.Errorf("%s: planexpiresat = %v; se esperaba %v", where, value, expires.Format(time.RFC3339))
	}
}

func TestLoginAndGetJwtReturnThePlan(t *testing.T) {
	db := mongotest.Connect(t)
	expires := time.Date(2027, 11, 10, 23, 59, 59, 0, time.UTC)
	tests := []struct {
		name       string
		email      string
		typeclient string
		expires    *time.Time
		// Tope propio de la cuenta (users.clientlimit) como se escriba a mano; nil: no tiene
		clientLimit any
		wantBackup  bool
		wantLimit   float64
	}{
		{"versión gratis, sin fecha", "gratis-login@ejemplo.mx", plans.Free, nil, nil, false, 50},
		{"plan de pago con vigencia", "pago-login@ejemplo.mx", plans.Paid, &expires, nil, true, 300},
		{"plan desconocido: como la versión gratis", "raro-login@ejemplo.mx", "Diamond", nil, nil, false, 50},
		{"plan de pago ampliado a 500", "ampliado-login@ejemplo.mx", plans.Paid, nil, int32(500), true, 500},
		{"tope propio guardado como decimal", "decimal-login@ejemplo.mx", plans.Paid, nil, float64(500), true, 500},
		{"versión gratis con tope propio: no cuenta", "gratis-tope@ejemplo.mx", plans.Free, nil, int32(500), false, 50},
		{"tope propio mal escrito: entra con el del plan", "texto-tope@ejemplo.mx", plans.Paid, nil, "500", true, 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := registerWithPassword(t, db, tt.email)
			setPlan(t, db, tt.email, tt.typeclient, tt.expires)
			if tt.clientLimit != nil {
				setClientLimit(t, db, tt.email, tt.clientLimit)
			}

			login := serveLogin(t, db, tt.email, "dispositivo-plan")
			checkPlan(t, "login", login, tt.typeclient, tt.wantBackup, tt.wantLimit, tt.expires)
			renewed := serveGetJwt(t, db, user, "dispositivo-plan", login)
			checkPlan(t, "/GetJwt", renewed, tt.typeclient, tt.wantBackup, tt.wantLimit, tt.expires)
		})
	}
}

// Pasar de la versión gratis al plan de pago (a mano en la base) le llega a la app
// en la siguiente renovación del token, sin volver a entrar.
func TestGetJwtBringsAPlanChange(t *testing.T) {
	db := mongotest.Connect(t)
	const email = "cambio@ejemplo.mx"
	user := registerWithPassword(t, db, email)
	login := serveLogin(t, db, email, "dispositivo-cambio")
	checkPlan(t, "login", login, plans.Free, false, 50, nil)

	expires := time.Date(2027, 11, 10, 23, 59, 59, 0, time.UTC)
	setPlan(t, db, email, plans.Paid, &expires)
	renewed := serveGetJwt(t, db, user, "dispositivo-cambio", login)
	checkPlan(t, "/GetJwt", renewed, plans.Paid, true, 300, &expires)
}

// Ampliarle el tope a una cuenta de pago (a mano en la base) le llega a la app en la
// siguiente renovación del token, sin volver a entrar; quitárselo la regresa al del plan.
func TestGetJwtBringsAnAccountLimitChange(t *testing.T) {
	db := mongotest.Connect(t)
	const email = "ampliacion@ejemplo.mx"
	const device = "dispositivo-ampliacion"
	user := registerWithPassword(t, db, email)
	setPlan(t, db, email, plans.Paid, nil)
	session := serveLogin(t, db, email, device)
	checkPlan(t, "login", session, plans.Paid, true, 300, nil)

	renew := func() map[string]any {
		renewed := serveGetJwt(t, db, user, device, session)
		// Si el servidor rotó el refresh token, la siguiente renovación usa el nuevo
		if rotated, ok := renewed["refreshtoken"].(string); ok && rotated != "" {
			session["refreshtoken"] = rotated
		}
		return renewed
	}

	setClientLimit(t, db, email, int32(500))
	checkPlan(t, "/GetJwt con tope propio", renew(), plans.Paid, true, 500, nil)

	setClientLimit(t, db, email, nil)
	checkPlan(t, "/GetJwt sin tope propio", renew(), plans.Paid, true, 300, nil)
}

// Nadie se pone el plan de pago, una vigencia ni su propio tope al registrarse.
func TestRegisterAlwaysStartsFree(t *testing.T) {
	db := mongotest.Connect(t)
	body := `{"name":"Ana","lastname":"López","email":"nueva@ejemplo.mx","password":"clave-de-prueba-1",
		"phone":2221234567,"termsaccepted":true,"termsversion":"2026-10-02",
		"typeclient":"Amethyst","planexpiresat":"2030-01-01T00:00:00Z","clientlimit":5000}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	Register(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("código = %d; se esperaba 201 (cuerpo: %s)", rec.Code, rec.Body)
	}
	user, err := db.FindUser(context.Background(), "nueva@ejemplo.mx")
	if err != nil {
		t.Fatalf("FindUser: %v", err)
	}
	if user.TypeClient != plans.Free || user.PlanExpiresAt != nil {
		t.Errorf("cuenta nueva: plan = %q, vigencia = %v; se esperaba %q sin fecha", user.TypeClient, user.PlanExpiresAt, plans.Free)
	}
	var stored bson.M
	if err := mongotest.Raw(t, db).Collection("users").FindOne(context.Background(), bson.M{"email": "nueva@ejemplo.mx"}).Decode(&stored); err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if value, ok := stored["clientlimit"]; ok {
		t.Errorf("la cuenta nueva quedó con tope propio: %v", value)
	}
}
