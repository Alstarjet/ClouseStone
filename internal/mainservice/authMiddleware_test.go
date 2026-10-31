package mainservice

import (
	"financial-Assistant/internal/mainservice/ctxkeys"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/utilities"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const testAccessSecret = "secreto-de-acceso-de-prueba"

// signToken firma claims arbitrarios con HS256 para armar tokens que
// utilities.GenerateToken no produce (vencidos, sin email, etc.).
func signToken(t *testing.T, claims jwt.MapClaims, secret string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("firmar token: %v", err)
	}
	return token
}

func TestAuthMiddleware(t *testing.T) {
	t.Setenv("ACCESS_SECRET", testAccessSecret)
	t.Setenv("ENVIRONMENT", "local")

	user := models.User{ID: primitive.NewObjectID(), Email: "ana@ejemplo.mx", TypeClient: "Quartz"}
	validToken, _, err := utilities.GenerateToken(user, "dispositivo-1", testAccessSecret)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	// Firmado con el mismo secreto: sólo el tipo ("refresh") lo invalida.
	refreshToken, _, err := utilities.GenerateRefreshToken(user, "dispositivo-1", testAccessSecret)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	accessClaims := func(exp time.Time) jwt.MapClaims {
		return jwt.MapClaims{"exp": exp.Unix(), "type": "access", "email": user.Email, "deviceid": "dispositivo-1"}
	}
	expiredToken := signToken(t, accessClaims(time.Now().Add(-time.Minute)), testAccessSecret)
	otherSecretToken := signToken(t, accessClaims(time.Now().Add(time.Minute)), "otro-secreto")
	withoutEmail := accessClaims(time.Now().Add(time.Minute))
	delete(withoutEmail, "email")
	withoutDevice := accessClaims(time.Now().Add(time.Minute))
	delete(withoutDevice, "deviceid")

	const (
		errMissing = `{"error":"missing Authorization header"}`
		errFormat  = `{"error":"invalid Authorization header format"}`
		errToken   = `{"error":"invalid or expired token"}`
		errClaims  = `{"error":"invalid token claims"}`
	)

	tests := []struct {
		name     string
		header   string // valor de Authorization; "" = sin header
		wantCode int
		wantBody string // sólo se revisa en los 401
	}{
		{"sin header", "", http.StatusUnauthorized, errMissing},
		{"sin esquema", validToken, http.StatusUnauthorized, errFormat},
		{"esquema distinto de Bearer", "Basic " + validToken, http.StatusUnauthorized, errFormat},
		{"Bearer sin token", "Bearer", http.StatusUnauthorized, errFormat},
		{"Bearer con espacios de más", "Bearer  " + validToken, http.StatusUnauthorized, errFormat},
		{"token que no es JWT", "Bearer abc.def.ghi", http.StatusUnauthorized, errToken},
		{"token vencido", "Bearer " + expiredToken, http.StatusUnauthorized, errToken},
		{"token firmado con otro secreto", "Bearer " + otherSecretToken, http.StatusUnauthorized, errToken},
		{"refresh token en lugar de access", "Bearer " + refreshToken, http.StatusUnauthorized, errToken},
		{"token sin email", "Bearer " + signToken(t, withoutEmail, testAccessSecret), http.StatusUnauthorized, errClaims},
		{"token sin deviceid", "Bearer " + signToken(t, withoutDevice, testAccessSecret), http.StatusUnauthorized, errClaims},
		{"token válido", "Bearer " + validToken, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotEmail, gotDevice any
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotEmail = r.Context().Value(ctxkeys.Email)
				gotDevice = r.Context().Value(ctxkeys.DeviceID)
			})

			req := httptest.NewRequest(http.MethodPost, "/UploadData", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			AuthMiddleware(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("código = %d; se esperaba %d (cuerpo: %s)", rec.Code, tt.wantCode, rec.Body)
			}
			if tt.wantCode != http.StatusOK {
				if nextCalled {
					t.Error("el handler protegido se ejecutó con un token rechazado")
				}
				if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
					t.Errorf("cuerpo = %s; se esperaba %s", got, tt.wantBody)
				}
				return
			}
			if !nextCalled {
				t.Fatal("el handler protegido no se ejecutó con un token válido")
			}
			if gotEmail != user.Email || gotDevice != "dispositivo-1" {
				t.Errorf("contexto = (%v, %v); se esperaba (%q, %q)", gotEmail, gotDevice, user.Email, "dispositivo-1")
			}
		})
	}
}

// /UploadData está registrada con AuthMiddleware: sin token no llega al handler
// (ni a la base) y sólo acepta POST.
func TestUploadDataRouteRequiresAuth(t *testing.T) {
	t.Setenv("ACCESS_SECRET", testAccessSecret)
	t.Setenv("ENVIRONMENT", "local")

	// Sin Mongo: si alguna petición llegara a la base, la prueba fallaría con un pánico.
	router := NewRouter(&Server{})
	user := models.User{ID: primitive.NewObjectID(), Email: "ana@ejemplo.mx"}
	token, _, err := utilities.GenerateToken(user, "dispositivo-1", testAccessSecret)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	tests := []struct {
		name     string
		method   string
		header   string
		body     string
		wantCode int
	}{
		{"POST sin token", http.MethodPost, "", `{"clients":[]}`, http.StatusUnauthorized},
		{"POST con token y cuerpo inválido", http.MethodPost, "Bearer " + token, `{"clients":[{"age":"40"}]}`, http.StatusBadRequest},
		{"GET no está permitido", http.MethodGet, "Bearer " + token, "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/UploadData", strings.NewReader(tt.body))
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("código = %d; se esperaba %d (cuerpo: %s)", rec.Code, tt.wantCode, rec.Body)
			}
		})
	}
}
