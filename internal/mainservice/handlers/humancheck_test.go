package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeTurnstile levanta un Cloudflare de mentiras que responde `answer` y anota lo
// que le preguntaron.
func fakeTurnstile(t *testing.T, status int, answer string) *[]string {
	t.Helper()
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("formulario ilegible: %v", err)
		}
		asked = append(asked, r.Form.Encode())
		w.WriteHeader(status)
		w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	previous := turnstileVerifyURL
	turnstileVerifyURL = server.URL
	t.Cleanup(func() { turnstileVerifyURL = previous })
	return &asked
}

func TestHumanCheck(t *testing.T) {
	const ok = `{"success":true}`
	const rejected = `{"success":false,"error-codes":["invalid-input-response"]}`
	tests := []struct {
		name    string
		secret  string
		body    string
		status  int
		answer  string
		wantErr error
		asks    int
	}{
		{"sin llave: funciona como antes", "", `{"email":"a@b.mx"}`, 200, rejected, nil, 0},
		{"sin llave, la trampa igual detiene al bot", "", `{"website":"http://spam.example"}`, 200, ok, errNotHuman, 0},
		{"prueba buena", "secreta", `{"turnstiletoken":"prueba-buena"}`, 200, ok, nil, 1},
		{"Cloudflare la rechaza", "secreta", `{"turnstiletoken":"usada"}`, 200, rejected, errNotHuman, 1},
		{"sin prueba", "secreta", `{"email":"a@b.mx"}`, 200, ok, errNotHuman, 0},
		{"prueba demasiado larga", "secreta", `{"turnstiletoken":"` + strings.Repeat("x", maxTurnstileTokenLength+1) + `"}`, 200, ok, errNotHuman, 0},
		{"trampa llena aunque traiga prueba", "secreta", `{"website":"x","turnstiletoken":"prueba-buena"}`, 200, ok, errNotHuman, 0},
		{"trampa con espacios cuenta como vacía", "secreta", `{"website":"  ","turnstiletoken":"prueba-buena"}`, 200, ok, nil, 1},
		{"Cloudflare falla", "secreta", `{"turnstiletoken":"prueba-buena"}`, 500, ``, errHumanCheckUnavailable, 1},
		{"respuesta ilegible", "secreta", `{"turnstiletoken":"prueba-buena"}`, 200, `no es json`, errHumanCheckUnavailable, 1},
		{"llave secreta equivocada", "secreta", `{"turnstiletoken":"prueba-buena"}`, 200, `{"success":false,"error-codes":["invalid-input-secret"]}`, errHumanCheckUnavailable, 1},
		{"tipo equivocado", "secreta", `{"turnstiletoken":123}`, 200, ok, errNotHuman, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(turnstileSecretEnv, tt.secret)
			asked := fakeTurnstile(t, tt.status, tt.answer)
			err := humanCheck(context.Background(), []byte(tt.body), "203.0.113.7")
			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("err = %v; se esperaba %v", err, tt.wantErr)
			}
			if len(*asked) != tt.asks {
				t.Fatalf("se le preguntó a Cloudflare %d veces; se esperaban %d", len(*asked), tt.asks)
			}
		})
	}
}

// A Cloudflare se le manda la llave, la prueba y la IP de quien se registra.
func TestHumanCheckSendsSecretTokenAndIP(t *testing.T) {
	t.Setenv(turnstileSecretEnv, "secreta")
	asked := fakeTurnstile(t, 200, `{"success":true}`)
	if err := humanCheck(context.Background(), []byte(`{"turnstiletoken":" prueba "}`), "203.0.113.7"); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "remoteip=203.0.113.7&response=prueba&secret=secreta"
	if (*asked)[0] != want {
		t.Errorf("formulario = %q; se esperaba %q", (*asked)[0], want)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/register", nil)
	r.RemoteAddr = "10.0.0.5:4321"
	if got := clientIP(r); got != "10.0.0.5" {
		t.Errorf("sin X-Forwarded-For = %q", got)
	}
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("con X-Forwarded-For = %q", got)
	}
	r.Header.Del("X-Forwarded-For")
	r.RemoteAddr = "[2001:db8::1]:4321"
	if got := clientIP(r); got != "2001:db8::1" {
		t.Errorf("IPv6 = %q", got)
	}
}

// El registro se detiene antes de tocar la base: un bot no llega ni a buscar el correo.
func TestRegisterStopsBotsBeforeTheDatabase(t *testing.T) {
	t.Setenv(turnstileSecretEnv, "secreta")
	fakeTurnstile(t, 200, `{"success":false,"error-codes":["invalid-input-response"]}`)
	valid := `"name":"Ana","email":"ana@ejemplo.mx","password":"12345678","termsaccepted":true,"termsversion":"2026-10-02"`
	tests := []struct {
		name   string
		body   string
		status int
		answer string
	}{
		{"prueba rechazada", `{` + valid + `,"turnstiletoken":"mala"}`, http.StatusBadRequest, "no eres un robot"},
		{"trampa llena", `{` + valid + `,"website":"spam"}`, http.StatusBadRequest, "no eres un robot"},
		{"sin prueba", `{` + valid + `}`, http.StatusBadRequest, "no eres un robot"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sin base: si el registro intentara usarla, la prueba truena
			handler := Register(nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.body)))
			if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.answer) {
				t.Fatalf("respuesta = %d %s; se esperaba %d con %q", w.Code, w.Body.String(), tt.status, tt.answer)
			}
		})
	}

	t.Run("Cloudflare no responde: 503, vuelve a intentar", func(t *testing.T) {
		fakeTurnstile(t, 502, ``)
		w := httptest.NewRecorder()
		Register(nil).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{`+valid+`,"turnstiletoken":"buena"}`)))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("respuesta = %d %s; se esperaba 503", w.Code, w.Body.String())
		}
	})
}
