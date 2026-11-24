// Package googleid verifica los tokens de identidad (OIDC) que firma Google, como el que
// manda Cloud Scheduler al llamar una ruta interna con su cuenta de servicio.
//
// Se hace con golang-jwt (ya en el proyecto) y las llaves públicas de Google, en lugar de
// google.golang.org/api/idtoken, que obliga a subir la versión de Go del módulo y trae
// muchas dependencias para una sola comprobación.
package googleid

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CertsURL son las llaves públicas con las que Google firma los tokens de identidad (JWKS).
const CertsURL = "https://www.googleapis.com/oauth2/v3/certs"

const (
	// Cuánto se guardan las llaves si Google no dice (Cache-Control: max-age), y el máximo.
	defaultMaxAge = time.Hour
	maxMaxAge     = 24 * time.Hour
	// Con un kid desconocido se vuelven a pedir las llaves (Google las rota), pero no más de
	// una vez por minuto: un token inventado no debe poder hacer que se pidan en cada llamada.
	refetchEvery = time.Minute
	maxCertsBody = 1 << 20
	leeway       = 30 * time.Second
)

// Los dos emisores con los que Google firma los tokens de identidad.
var issuers = map[string]bool{"https://accounts.google.com": true, "accounts.google.com": true}

// Claims es lo que importa de un token de identidad de Google.
type Claims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	jwt.RegisteredClaims
}

// Verifier verifica tokens de identidad de Google. Guarda las llaves en memoria.
type Verifier struct {
	certsURL string
	client   *http.Client

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	expires   time.Time
}

// NewVerifier crea un verificador que toma las llaves de certsURL (CertsURL en producción).
func NewVerifier(certsURL string, client *http.Client) *Verifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Verifier{certsURL: certsURL, client: client}
}

// Verify comprueba la firma (RS256 con una llave de Google), el emisor, la audiencia y que no
// haya vencido. No revisa el correo: eso le toca a quien llama.
func (v *Verifier) Verify(ctx context.Context, raw, audience string) (*Claims, error) {
	if audience == "" {
		return nil, errors.New("googleid: falta la audiencia")
	}
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(leeway),
	)
	if err != nil {
		return nil, fmt.Errorf("googleid: %w", err)
	}
	if !issuers[claims.Issuer] {
		return nil, fmt.Errorf("googleid: emisor inesperado %q", claims.Issuer)
	}
	return claims, nil
}

// key regresa la llave pública con ese kid; si no la tiene (o ya venció lo guardado), las pide.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if kid == "" {
		return nil, errors.New("el token no trae kid")
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	if key, ok := v.keys[kid]; ok && now.Before(v.expires) {
		return key, nil
	}
	if v.keys != nil && now.Before(v.expires) && now.Sub(v.fetchedAt) < refetchEvery {
		return nil, fmt.Errorf("llave desconocida %q", kid)
	}
	keys, maxAge, err := v.fetch(ctx)
	if err != nil {
		return nil, err
	}
	v.keys, v.fetchedAt, v.expires = keys, now, now.Add(maxAge)
	if key, ok := keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("llave desconocida %q", kid)
}

type jwks struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *Verifier) fetch(ctx context.Context) (map[string]*rsa.PublicKey, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("pedir las llaves de Google: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("llaves de Google: estado %d", resp.StatusCode)
	}
	var set jwks
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCertsBody)).Decode(&set); err != nil {
		return nil, 0, fmt.Errorf("leer las llaves de Google: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		key, err := rsaKey(k.N, k.E)
		if err != nil {
			return nil, 0, fmt.Errorf("llave %q: %w", k.Kid, err)
		}
		keys[k.Kid] = key
	}
	if len(keys) == 0 {
		return nil, 0, errors.New("Google no mandó ninguna llave RSA")
	}
	return keys, maxAge(resp.Header.Get("Cache-Control")), nil
}

func rsaKey(n, e string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("módulo: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("exponente: %w", err)
	}
	exp := new(big.Int).SetBytes(eb)
	if len(nb) < 256 || !exp.IsInt64() || exp.Int64() < 3 || exp.Int64() > 1<<31 {
		return nil, errors.New("llave RSA inválida")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(exp.Int64())}, nil
}

// maxAge lee el max-age de Cache-Control; sin él, una hora, y nunca más de un día.
func maxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if value, ok := strings.CutPrefix(part, "max-age="); ok {
			if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
				return min(time.Duration(seconds)*time.Second, maxMaxAge)
			}
		}
	}
	return defaultMaxAge
}
