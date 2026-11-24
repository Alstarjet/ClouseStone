package googleid

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testAudience = "https://api.ejemplo.mx/internal/plans/expire"

type certServer struct {
	*httptest.Server
	hits atomic.Int32
}

// newCertServer sirve las llaves públicas como lo hace Google (JWKS), con max-age de una hora.
func newCertServer(t *testing.T, keys map[string]*rsa.PublicKey) *certServer {
	t.Helper()
	cs := &certServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.hits.Add(1)
		var set jwks
		for kid, key := range keys {
			set.Keys = append(set.Keys, struct {
				Kid string `json:"kid"`
				Kty string `json:"kty"`
				N   string `json:"n"`
				E   string `json:"e"`
			}{kid, "RSA", base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(cs.Close)
	return cs
}

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generar llave: %v", err)
	}
	return key
}

func claimsFor(aud, iss string, exp time.Time) Claims {
	return Claims{
		Email:         "gluzie-scheduler@proyecto.iam.gserviceaccount.com",
		EmailVerified: true,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    iss,
			Audience:  jwt.ClaimStrings{aud},
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
}

func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("firmar: %v", err)
	}
	return raw
}

func TestVerify(t *testing.T) {
	google, other := newKey(t), newKey(t)
	certs := newCertServer(t, map[string]*rsa.PublicKey{"llave-1": &google.PublicKey})
	inAnHour := time.Now().Add(time.Hour)
	good := claimsFor(testAudience, "https://accounts.google.com", inAnHour)

	hs256, err := jwt.NewWithClaims(jwt.SigningMethodHS256, good).SignedString([]byte("cualquier-secreto"))
	if err != nil {
		t.Fatal(err)
	}
	valid := sign(t, google, "llave-1", good)
	tampered := valid[:len(valid)-4] + "AAAA"

	tests := []struct {
		name, token string
		ok          bool
	}{
		{"firmado por Google", valid, true},
		{"emisor sin https", sign(t, google, "llave-1", claimsFor(testAudience, "accounts.google.com", inAnHour)), true},
		{"otra audiencia", sign(t, google, "llave-1", claimsFor("https://otra.mx/ruta", "https://accounts.google.com", inAnHour)), false},
		{"otro emisor", sign(t, google, "llave-1", claimsFor(testAudience, "https://otro.mx", inAnHour)), false},
		{"vencido", sign(t, google, "llave-1", claimsFor(testAudience, "https://accounts.google.com", time.Now().Add(-time.Hour))), false},
		{"firmado con otra llave", sign(t, other, "llave-1", good), false},
		{"kid que Google no tiene", sign(t, other, "llave-falsa", good), false},
		{"firma alterada", tampered, false},
		{"HS256 (confusión de algoritmo)", hs256, false},
		{"basura", "esto.no.es-un-token", false},
	}
	verifier := NewVerifier(certs.URL, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := verifier.Verify(context.Background(), tt.token, testAudience)
			if tt.ok && err != nil {
				t.Fatalf("se esperaba válido: %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("se esperaba rechazo; claims = %+v", claims)
			}
			if tt.ok && (claims.Email != good.Email || !claims.EmailVerified) {
				t.Errorf("claims = %+v", claims)
			}
		})
	}
}

func TestVerifyWithoutAudience(t *testing.T) {
	key := newKey(t)
	certs := newCertServer(t, map[string]*rsa.PublicKey{"k": &key.PublicKey})
	token := sign(t, key, "k", claimsFor("", "https://accounts.google.com", time.Now().Add(time.Hour)))
	if _, err := NewVerifier(certs.URL, nil).Verify(context.Background(), token, ""); err == nil {
		t.Fatal("sin audiencia configurada no debe aceptar nada")
	}
}

// Las llaves se guardan: no se piden en cada llamada, ni siquiera con kids inventados.
func TestVerifyCachesKeys(t *testing.T) {
	key := newKey(t)
	certs := newCertServer(t, map[string]*rsa.PublicKey{"k": &key.PublicKey})
	verifier := NewVerifier(certs.URL, nil)
	good := claimsFor(testAudience, "https://accounts.google.com", time.Now().Add(time.Hour))
	for i := 0; i < 5; i++ {
		if _, err := verifier.Verify(context.Background(), sign(t, key, "k", good), testAudience); err != nil {
			t.Fatalf("vuelta %d: %v", i, err)
		}
		_, _ = verifier.Verify(context.Background(), sign(t, key, "inventado", good), testAudience)
	}
	if hits := certs.hits.Load(); hits != 1 {
		t.Errorf("se pidieron las llaves %d veces; se esperaba 1", hits)
	}
}

func TestMaxAge(t *testing.T) {
	tests := map[string]time.Duration{
		"public, max-age=19800, must-revalidate": 19800 * time.Second,
		"":                                       defaultMaxAge,
		"max-age=abc":                            defaultMaxAge,
		"max-age=999999":                         maxMaxAge,
	}
	for header, want := range tests {
		if got := maxAge(header); got != want {
			t.Errorf("maxAge(%q) = %v; se esperaba %v", header, got, want)
		}
	}
}

func TestRejectsShortKeys(t *testing.T) {
	if _, err := rsaKey(base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("a", 64))), "AQAB"); err == nil {
		t.Error("una llave de 512 bits no debe aceptarse")
	}
}
