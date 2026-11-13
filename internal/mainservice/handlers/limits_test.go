package handlers

import (
	"context"
	"errors"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// Topes y freno de intentos de la evaluación de seguridad del 12-nov-2026
// (S-02, S-03 y S-05). Nada de esto necesita MongoDB.

// --- S-02: topes de tamaño en /register y /login -----------------------------

func TestRegisterLimits(t *testing.T) {
	ok := models.User{Name: strings.Repeat("ñ", 80), LastName: strings.Repeat("a", 80),
		Email: strings.Repeat("a", 244) + "@gluzie.mx", Password: strings.Repeat("p", 72)}
	if msg := registerLimits(ok); msg != "" {
		t.Fatalf("en el tope exacto se rechazó: %s", msg)
	}
	tests := []struct {
		name   string
		change func(*models.User)
		want   string
	}{
		{"nombre de 81", func(u *models.User) { u.Name += "x" }, "nombre"},
		{"apellido de 81", func(u *models.User) { u.LastName += "x" }, "apellido"},
		{"correo de 255", func(u *models.User) { u.Email = "x" + u.Email }, "email"},
		{"contraseña de 73 bytes", func(u *models.User) { u.Password += "x" }, "contraseña"},
		{"contraseña de 72 letras con acento", func(u *models.User) { u.Password = strings.Repeat("p", 71) + "ñ" }, "contraseña"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := ok
			tt.change(&user)
			if msg := registerLimits(user); !strings.Contains(msg, tt.want) {
				t.Errorf("mensaje = %q; se esperaba uno sobre %q", msg, tt.want)
			}
		})
	}
}

// serveRegister manda body a /register sin base: si el handler llegara a usarla,
// la prueba falla en vez de entrar en pánico.
func serveRegister(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("el registro llegó a la base de datos: %v", r)
		}
	}()
	Register(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body)))
	return rec
}

// Un nombre gigante, o un cuerpo de más de 16 KB, se rechaza antes de tocar la
// base: ya no se puede guardar un documento de 1 MB por registro.
func TestRegisterRejectsOversizedBeforeTheDatabase(t *testing.T) {
	long := fmt.Sprintf(`{"name":%q,"lastname":"López","email":"ana@ejemplo.mx","password":"clave-de-prueba-1"}`,
		strings.Repeat("a", 81))
	rec := serveRegister(t, long)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "nombre es demasiado largo") {
		t.Errorf("nombre de 81: %d %s", rec.Code, rec.Body)
	}

	huge := fmt.Sprintf(`{"name":"Ana","lastname":%q,"email":"ana@ejemplo.mx","password":"clave-de-prueba-1"}`,
		strings.Repeat("a", 20<<10))
	rec = serveRegister(t, huge)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "error al leer la solicitud") {
		t.Errorf("cuerpo de 20 KB: %d %s", rec.Code, rec.Body)
	}
}

// --- S-03: freno de intentos de /login ----------------------------------------

// freshLoginLimits deja el freno en cero para la prueba, con un reloj que la
// prueba mueve, y lo regresa al terminar.
func freshLoginLimits(t *testing.T) *time.Time {
	t.Helper()
	previous := loginLimits
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	loginLimits = newLoginLimiter()
	loginLimits.byIP.now = func() time.Time { return now }
	loginLimits.byAccount.now = func() time.Time { return now }
	t.Cleanup(func() { loginLimits = previous })
	return &now
}

// tryLogin corre authenticateUser como lo hace /login y regresa el código (200
// si las credenciales pasaron).
func tryLogin(finder userFinder, ip, email, password string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"email":%q,"password":%q,"device":"dispositivo-1"}`, email, password)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("X-Forwarded-For", ip)
	rec := httptest.NewRecorder()
	authenticateUser(context.Background(), finder, rec, req)
	return rec
}

// oneAccount es una base con una sola cuenta, ana@ejemplo.mx con testPassword;
// cuenta cuántas veces se buscó.
type oneAccount struct {
	user    models.User
	lookups int
	fail    error
}

func newOneAccount(t *testing.T) *oneAccount {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return &oneAccount{user: models.User{ID: primitive.NewObjectID(), Email: testEmail, Password: string(hashed)}}
}

func (o *oneAccount) FindUser(_ context.Context, email string) (models.User, error) {
	o.lookups++
	if o.fail != nil {
		return models.User{}, o.fail
	}
	if email != o.user.Email {
		return models.User{}, mongo.ErrNoDocuments
	}
	return o.user, nil
}

// Diez contraseñas equivocadas contra una cuenta, cada una desde otra IP: la
// cuenta queda frenada para todos, también con la contraseña correcta, sin tocar
// la base; pasada la ventana vuelve a entrar.
func TestLoginStopsGuessingOneAccount(t *testing.T) {
	now := freshLoginLimits(t)
	db := newOneAccount(t)

	for i := 1; i <= maxLoginFailsAccount; i++ {
		if rec := tryLogin(db, fmt.Sprintf("203.0.113.%d", i), testEmail, "adivinanza"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("intento %d: código = %d; se esperaba 401", i, rec.Code)
		}
	}
	lookups := db.lookups
	rec := tryLogin(db, "198.51.100.7", testEmail, testPassword)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("con la cuenta frenada: código = %d; se esperaba 429 (%s)", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"too_many_attempts"`) || rec.Header().Get("Retry-After") != "901" {
		t.Errorf("respuesta = %s, Retry-After = %q; se esperaba too_many_attempts y 901", rec.Body, rec.Header().Get("Retry-After"))
	}
	if db.lookups != lookups {
		t.Errorf("la cuenta frenada todavía consultó la base (%d veces)", db.lookups-lookups)
	}
	// Otra cuenta, desde una IP sin fallos, no se entera
	if rec := tryLogin(db, "198.51.100.7", "otra@ejemplo.mx", "x"); rec.Code != http.StatusUnauthorized {
		t.Errorf("otra cuenta: código = %d; se esperaba 401", rec.Code)
	}

	*now = now.Add(loginWindow - time.Second)
	if rec := tryLogin(db, "198.51.100.7", testEmail, testPassword); rec.Code != http.StatusTooManyRequests {
		t.Errorf("un segundo antes de terminar la ventana: código = %d; se esperaba 429", rec.Code)
	}
	*now = now.Add(time.Second)
	if rec := tryLogin(db, "198.51.100.7", testEmail, testPassword); rec.Code != http.StatusOK {
		t.Errorf("terminada la ventana: código = %d; se esperaba entrar (%s)", rec.Code, rec.Body)
	}
}

// Veinte fallos desde una IP, cada uno contra otra cuenta: esa IP queda frenada
// para cualquier cuenta, y las demás IP siguen entrando.
func TestLoginStopsOneIPTryingManyAccounts(t *testing.T) {
	freshLoginLimits(t)
	db := newOneAccount(t)

	for i := 1; i <= maxLoginFailsIP; i++ {
		if rec := tryLogin(db, "203.0.113.9", fmt.Sprintf("cuenta%d@ejemplo.mx", i), "adivinanza"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("intento %d: código = %d; se esperaba 401", i, rec.Code)
		}
	}
	if rec := tryLogin(db, "203.0.113.9", testEmail, testPassword); rec.Code != http.StatusTooManyRequests {
		t.Errorf("desde la IP frenada: código = %d; se esperaba 429", rec.Code)
	}
	if rec := tryLogin(db, "203.0.113.10", testEmail, testPassword); rec.Code != http.StatusOK {
		t.Errorf("desde otra IP: código = %d; se esperaba entrar (%s)", rec.Code, rec.Body)
	}
}

// Quien se equivoca unas veces y luego acierta empieza de cero; y una caída de la
// base (500) no cuenta como intento fallido.
func TestLoginForgivesAfterSuccessAndDatabaseErrors(t *testing.T) {
	freshLoginLimits(t)
	db := newOneAccount(t)

	for round := 0; round < 3; round++ {
		for i := 1; i < maxLoginFailsAccount; i++ {
			tryLogin(db, fmt.Sprintf("203.0.113.%d", i), testEmail, "se me olvidó")
		}
		if rec := tryLogin(db, "198.51.100.7", testEmail, testPassword); rec.Code != http.StatusOK {
			t.Fatalf("vuelta %d: código = %d; con 9 fallos todavía debe entrar", round, rec.Code)
		}
	}

	db.fail = errors.New("server selection timeout")
	for i := 0; i < maxLoginFailsIP+5; i++ {
		if rec := tryLogin(db, "192.0.2.50", testEmail, testPassword); rec.Code != http.StatusInternalServerError {
			t.Fatalf("con la base caída: código = %d; se esperaba 500", rec.Code)
		}
	}
	db.fail = nil
	if rec := tryLogin(db, "192.0.2.50", testEmail, testPassword); rec.Code != http.StatusOK {
		t.Errorf("al volver la base: código = %d; los 500 no deben frenar a nadie", rec.Code)
	}
}

// El primer valor de X-Forwarded-For lo escribe el cliente: inventar uno distinto
// en cada intento no esquiva el freno.
func TestLoginLimitIgnoresForgedForwardedFor(t *testing.T) {
	freshLoginLimits(t)
	db := newOneAccount(t)

	for i := 1; i <= maxLoginFailsIP; i++ {
		forged := fmt.Sprintf("10.9.8.%d, 203.0.113.9", i)
		tryLogin(db, forged, fmt.Sprintf("cuenta%d@ejemplo.mx", i), "adivinanza")
	}
	if rec := tryLogin(db, "10.1.1.1, 203.0.113.9", testEmail, testPassword); rec.Code != http.StatusTooManyRequests {
		t.Errorf("con IP inventada delante: código = %d; se esperaba 429", rec.Code)
	}
}

func TestRateLimitIP(t *testing.T) {
	tests := []struct{ forwarded, remote, want string }{
		{"203.0.113.9", "10.0.0.1:1234", "203.0.113.9"},
		{"1.2.3.4, 203.0.113.9", "10.0.0.1:1234", "203.0.113.9"},
		{" 1.2.3.4 ,5.6.7.8 , 203.0.113.9 ", "10.0.0.1:1234", "203.0.113.9"},
		{"", "192.0.2.1:1234", "192.0.2.1"},
		{"", "[2001:db8::1]:1234", "2001:db8::/64"},
		{"2001:db8:1:2:aaaa:bbbb:cccc:dddd", "10.0.0.1:1", "2001:db8:1:2::/64"},
		{"2001:db8:1:2:1111:2222:3333:4444", "10.0.0.1:1", "2001:db8:1:2::/64"},
		{"::ffff:203.0.113.9", "10.0.0.1:1", "203.0.113.9"},
		{"no-es-una-ip", "10.0.0.1:1", "no-es-una-ip"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = tt.remote
		if tt.forwarded != "" {
			req.Header.Set("X-Forwarded-For", tt.forwarded)
		}
		if got := rateLimitIP(req); got != tt.want {
			t.Errorf("X-Forwarded-For %q, conexión %q: IP = %q; se esperaba %q", tt.forwarded, tt.remote, got, tt.want)
		}
	}
}

// El contador no crece sin tope: con las llaves llenas quita las vencidas y, si
// todas siguen vivas, empieza de cero.
func TestAttemptLimiterMemoryIsBounded(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	limiter := newAttemptLimiter(3, time.Minute)
	limiter.now = func() time.Time { return now }

	for i := 0; i < maxLimiterKeys; i++ {
		limiter.fail(fmt.Sprintf("llave-%d", i))
	}
	now = now.Add(2 * time.Minute)
	limiter.fail("nueva")
	if n := len(limiter.entries); n != 1 {
		t.Errorf("después de vencer todas quedaron %d llaves; se esperaba 1", n)
	}

	for i := 0; i < maxLimiterKeys+10; i++ {
		limiter.fail(fmt.Sprintf("viva-%d", i))
	}
	if n := len(limiter.entries); n > maxLimiterKeys {
		t.Errorf("el contador llegó a %d llaves; el tope es %d", n, maxLimiterKeys)
	}
}

// Un dispositivo de más de 64 caracteres no entra, y un correo o una contraseña
// que no caben en una cuenta real ni siquiera se buscan.
func TestLoginRejectsOversizedFields(t *testing.T) {
	freshLoginLimits(t)
	db := newOneAccount(t)

	body := fmt.Sprintf(`{"email":%q,"password":%q,"device":%q}`, testEmail, testPassword, strings.Repeat("d", 65))
	rec := httptest.NewRecorder()
	authenticateUser(context.Background(), db, rec, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid device identifier") {
		t.Errorf("dispositivo de 65: %d %s", rec.Code, rec.Body)
	}

	for name, try := range map[string][2]string{
		"correo de 255":      {strings.Repeat("a", 245) + "@gluzie.mx", testPassword},
		"contraseña de 73 B": {testEmail, strings.Repeat("p", 73)},
	} {
		lookups := db.lookups
		if rec := tryLogin(db, "203.0.113.9", try[0], try[1]); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: código = %d; se esperaba 401", name, rec.Code)
		}
		if db.lookups != lookups {
			t.Errorf("%s: se buscó en la base", name)
		}
	}
	if n := len(loginLimits.byAccount.entries); n != 0 {
		t.Errorf("el correo gigante quedó guardado en el contador (%d llaves)", n)
	}
}

// --- S-05: tope de documentos por cuenta en /UploadData -----------------------

// Una cuenta que ya llegó al tope en una colección del lote recibe 413 y no se
// guarda nada, ni lo de las otras colecciones. Uno menos, y se guarda todo.
func TestUploadDataStopsAtTheAccountLimit(t *testing.T) {
	store := newFakeStore("Amethyst")
	store.counts = map[string]int64{"payments": database.MaxBackupRecords}
	rec := serveFake(store, validBackup)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("en el tope: código = %d; se esperaba 413 (%s)", rec.Code, rec.Body)
	}
	for _, want := range []string{`"error":"account_limit"`, `"store":"payments"`, `"limit":25000`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("respuesta = %s; falta %s", rec.Body, want)
		}
	}
	if len(store.writes) != 0 {
		t.Errorf("en el tope se escribió %v", store.written())
	}

	store = newFakeStore("Amethyst")
	store.counts = map[string]int64{"payments": database.MaxBackupRecords - 1}
	if rec := serveFake(store, validBackup); rec.Code != http.StatusOK {
		t.Fatalf("uno antes del tope: código = %d; se esperaba 200 (%s)", rec.Code, rec.Body)
	}
	if len(store.written()) != len(goodUUIDs) {
		t.Errorf("uno antes del tope se escribió %v; se esperaban las 4 colecciones", store.written())
	}
}

// Sólo se cuentan las colecciones que trae el lote (un pago suelto es una sola
// consulta), y si no se pudo contar es 500: el teléfono reintenta.
func TestUploadDataCountsOnlyWhatItBrings(t *testing.T) {
	store := newFakeStore("Amethyst")
	store.counts = map[string]int64{"clients": database.MaxBackupRecords}
	onlyPayment := `{"payments":[{"uuid":"pag-9","clientuuid":"cli-1","amount":50}]}`
	if rec := serveFake(store, onlyPayment); rec.Code != http.StatusOK {
		t.Fatalf("un pago suelto: código = %d; el tope de clientes no le toca (%s)", rec.Code, rec.Body)
	}
	if len(store.counted) != 1 || store.counted[0] != "payments" {
		t.Errorf("se contaron %v; se esperaba sólo payments", store.counted)
	}

	store = newFakeStore("Amethyst")
	store.countErr = errors.New("server selection timeout")
	if rec := serveFake(store, validBackup); rec.Code != http.StatusInternalServerError {
		t.Errorf("sin poder contar: código = %d; se esperaba 500", rec.Code)
	}
	if len(store.writes) != 0 {
		t.Errorf("sin poder contar se escribió %v", store.written())
	}
}
