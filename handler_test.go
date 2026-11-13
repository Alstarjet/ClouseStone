package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const origenFront = "https://www.gluzie.com"

// respuestaGrande imita el JSON de /login con una cuenta completa (unos 32 KB).
var respuestaGrande = `{"clients":"` + strings.Repeat("cliente-", 4000) + `"}`

// rutasDePrueba responde un JSON grande en /grande, uno chico en /chico y, en
// /plazos, si pudo alargar los plazos de la conexión como lo hace withTimeLimit
// (internal/mainservice/timelimit.go).
func rutasDePrueba() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/grande", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, respuestaGrande)
	})
	mux.HandleFunc("/chico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"token":"abc"}`)
	})
	mux.HandleFunc("/plazos", func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		limite := time.Now().Add(time.Minute)
		if err := rc.SetReadDeadline(limite); err != nil {
			http.Error(w, "plazo de lectura: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := rc.SetWriteDeadline(limite); err != nil {
			http.Error(w, "plazo de escritura: "+err.Error(), http.StatusInternalServerError)
			return
		}
		io.WriteString(w, "ok")
	})
	return mux
}

func pedir(t *testing.T, metodo, ruta string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(metodo, ruta, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	newHandler(rutasDePrueba(), origenFront).ServeHTTP(w, r)
	return w
}

func TestPreguntaPreviaDeCORSQuedaEnCache(t *testing.T) {
	w := pedir(t, http.MethodOptions, "/grande", map[string]string{
		"Origin":                         origenFront,
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "authorization,content-type",
	})
	if got := w.Header().Get("Access-Control-Max-Age"); got != "7200" {
		t.Fatalf("Access-Control-Max-Age = %q, se esperaba 7200", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != origenFront {
		t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba %q", got, origenFront)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, se esperaba true", got)
	}
}

func TestOtroOrigenNoRecibePermiso(t *testing.T) {
	w := pedir(t, http.MethodOptions, "/grande", map[string]string{
		"Origin":                        "https://otro-sitio.example",
		"Access-Control-Request-Method": http.MethodPost,
	})
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("otro origen recibió Access-Control-Allow-Origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got != "" {
		t.Fatalf("otro origen recibió Access-Control-Max-Age = %q", got)
	}
}

func TestComprimeLasRespuestasGrandes(t *testing.T) {
	// Lo que manda Chrome
	w := pedir(t, http.MethodGet, "/grande", map[string]string{
		"Origin":          origenFront,
		"Accept-Encoding": "gzip, deflate, br, zstd",
	})
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, se esperaba gzip", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != origenFront {
		t.Fatalf("la respuesta comprimida perdió CORS: Access-Control-Allow-Origin = %q", got)
	}
	if vary := strings.Join(w.Header().Values("Vary"), ","); !strings.Contains(vary, "Accept-Encoding") {
		t.Fatalf("Vary = %q, falta Accept-Encoding", vary)
	}
	if w.Body.Len() >= len(respuestaGrande)/4 {
		t.Fatalf("comprimió poco: %d de %d bytes", w.Body.Len(), len(respuestaGrande))
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("no es gzip válido: %v", err)
	}
	cuerpo, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("no se pudo descomprimir: %v", err)
	}
	if string(cuerpo) != respuestaGrande {
		t.Fatalf("el cuerpo descomprimido no coincide (%d bytes)", len(cuerpo))
	}
}

func TestSinGzipSaleIgualQueAntes(t *testing.T) {
	w := pedir(t, http.MethodGet, "/grande", map[string]string{"Origin": origenFront})
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q sin que el cliente lo pidiera", got)
	}
	if w.Body.String() != respuestaGrande {
		t.Fatalf("el cuerpo cambió (%d bytes)", w.Body.Len())
	}
}

func TestRespuestaChicaSaleSinComprimir(t *testing.T) {
	w := pedir(t, http.MethodGet, "/chico", map[string]string{
		"Origin":          origenFront,
		"Accept-Encoding": "gzip",
	})
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q en una respuesta de menos de 1 KB", got)
	}
	if got := w.Body.String(); got != `{"token":"abc"}` {
		t.Fatalf("cuerpo = %q", got)
	}
}

// Con una conexión real, como en Cloud Run: withTimeLimit alarga los plazos con
// http.ResponseController, que tiene que llegar a la conexión a través de la
// compresión. Con gzhttp de klauspost/compress v1.13 no llegaba, y los plazos de
// /UploadData y /login se perdían sin error (I-03).
func TestLosPlazosLleganALaConexion(t *testing.T) {
	srv := httptest.NewServer(newHandler(rutasDePrueba(), origenFront))
	defer srv.Close()
	for _, enc := range []string{"gzip", "identity"} {
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/plazos", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept-Encoding", enc)
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("Accept-Encoding %q: %v", enc, err)
		}
		cuerpo, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK || string(cuerpo) != "ok" {
			t.Fatalf("Accept-Encoding %q: %d %q", enc, res.StatusCode, strings.TrimSpace(string(cuerpo)))
		}
	}
}

// MaxAge sólo va en la respuesta al OPTIONS. Lo que el navegador guarda es el
// permiso, no los datos: cada respaldo y cada descarga sigue llegando a la ruta,
// y su respuesta lleva sus propios permisos de CORS, sin caché y sin cambios.
func TestPeticionRealSiempreLlegaYNoCambia(t *testing.T) {
	llamadas := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/GetDataSince", func(w http.ResponseWriter, r *http.Request) {
		llamadas++
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, respuestaGrande)
	})
	h := newHandler(mux, origenFront)
	for i := 1; i <= 3; i++ {
		// Como la arma la app (src/backend/getData.ts y authFetch.ts)
		r := httptest.NewRequest(http.MethodGet, "/GetDataSince?since=2026-11-12T00%3A00%3A00Z", nil)
		r.Header.Set("Origin", origenFront)
		r.Header.Set("Authorization", "Bearer token-de-prueba")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if llamadas != i {
			t.Fatalf("la petición %d no llegó a la ruta (llamadas = %d)", i, llamadas)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origenFront {
			t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba %q", got, origenFront)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("Access-Control-Allow-Credentials = %q, se esperaba true", got)
		}
		if got := w.Header().Get("Access-Control-Max-Age"); got != "" {
			t.Fatalf("la respuesta real trae Access-Control-Max-Age = %q", got)
		}
		if got := w.Header().Get("Cache-Control"); got != "" {
			t.Fatalf("la respuesta real trae Cache-Control = %q", got)
		}
		if w.Body.String() != respuestaGrande {
			t.Fatalf("el cuerpo cambió (%d bytes)", w.Body.Len())
		}
	}
}
