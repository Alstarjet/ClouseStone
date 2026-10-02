package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/catalog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
)

type stubLoader struct {
	docs models.CatalogDocs
	err  error
}

func (s stubLoader) LoadCatalogDocs(context.Context) (models.CatalogDocs, error) {
	return s.docs, s.err
}

func catalogDocs() models.CatalogDocs {
	return models.CatalogDocs{
		Products: []bson.M{
			{"slug": "shampoo-bergamota", "key": "S1012", "displayName": "Shampoo de Bergamota", "productType": "shampoo",
				"rubros": bson.A{"belleza"}, "variants": bson.A{bson.M{"key": "S1012", "label": "", "image": "http://img/1"}},
				"needs": bson.A{bson.M{"need": "caida-del-cabello", "relevance": 1.0, "role": "principal"}}},
			{"slug": "gomitas-biotina", "key": "S1202", "displayName": "Gomitas con Biotina", "productType": "gomitas",
				"rubros": bson.A{"salud"}, "variants": bson.A{bson.M{"key": "S1202"}},
				"needs": bson.A{bson.M{"need": "caida-del-cabello", "relevance": 0.7, "role": "complemento"}}},
		},
		Kits: []bson.M{
			{"slug": "kit-caida", "title": "Recupera tu cabello", "needs": bson.A{"caida-del-cabello"}, "perfiles": bson.A{"mama-primeriza"},
				"items": bson.A{bson.M{"key": "S1012", "order": 1, "role": "principal"}, bson.M{"key": "S1202", "order": 2, "role": "complemento"}}},
		},
	}
}

// newCatalogRouter monta las mismas rutas que route.go, sin el middleware de
// autenticación (que se prueba aparte), sobre un servicio con datos de ejemplo.
func newCatalogRouter(loader catalog.Loader) *mux.Router {
	svc := catalog.NewService(loader, time.Minute)
	r := mux.NewRouter().StrictSlash(true)
	r.Handle("/catalog/products", CatalogProducts(svc)).Methods(http.MethodGet)
	r.Handle("/catalog/products/{id}", CatalogProduct(svc)).Methods(http.MethodGet)
	r.Handle("/catalog/kits", CatalogKits(svc)).Methods(http.MethodGet)
	r.Handle("/catalog/kits/{slug}", CatalogKit(svc)).Methods(http.MethodGet)
	return r
}

func get(t *testing.T, router http.Handler, url string) (int, map[string]any, http.Header) {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: la respuesta no es JSON: %v\n%s", url, err, rec.Body.String())
	}
	return rec.Code, body, rec.Header()
}

func TestCatalogHandlers(t *testing.T) {
	router := newCatalogRouter(stubLoader{docs: catalogDocs()})

	tests := []struct {
		name   string
		url    string
		status int
		check  func(t *testing.T, body map[string]any)
	}{
		{"lista de productos con envoltura", "/catalog/products", 200, func(t *testing.T, b map[string]any) {
			if b["total"] != 2.0 || b["limit"] != 50.0 || b["skip"] != 0.0 || len(b["items"].([]any)) != 2 {
				t.Errorf("envoltura = %v", b)
			}
		}},
		{"productos por necesidad ordenados por relevancia", "/catalog/products?need=caida-del-cabello", 200, func(t *testing.T, b map[string]any) {
			first := b["items"].([]any)[0].(map[string]any)
			if first["slug"] != "shampoo-bergamota" || first["score"] != 1.0 {
				t.Errorf("primero = %v", first)
			}
		}},
		{"paginación", "/catalog/products?limit=1&skip=1", 200, func(t *testing.T, b map[string]any) {
			if len(b["items"].([]any)) != 1 || b["total"] != 2.0 {
				t.Errorf("página = %v", b)
			}
		}},
		{"limit mayor al máximo se acota a 200", "/catalog/products?limit=9999", 200, func(t *testing.T, b map[string]any) {
			if b["limit"] != 200.0 {
				t.Errorf("limit = %v", b["limit"])
			}
		}},
		{"necesidad desconocida es 400", "/catalog/products?need=nope", 400, func(t *testing.T, b map[string]any) {
			if b["error"] == nil {
				t.Error("falta el mensaje de error")
			}
		}},
		{"limit no numérico es 400", "/catalog/products?limit=abc", 400, nil},
		{"min_relevance fuera de rango es 400", "/catalog/products?min_relevance=2", 400, nil},
		{"producto por clave", "/catalog/products/S1012", 200, func(t *testing.T, b map[string]any) {
			if b["slug"] != "shampoo-bergamota" || b["variant"] == nil || len(b["kits"].([]any)) != 1 {
				t.Errorf("doc = %v", b)
			}
		}},
		{"producto inexistente es 404", "/catalog/products/S0000", 404, nil},
		{"kits con productos resueltos", "/catalog/kits", 200, func(t *testing.T, b map[string]any) {
			items := b["items"].([]any)[0].(map[string]any)["items"].([]any)
			if items[0].(map[string]any)["product"] == nil {
				t.Errorf("items = %v", items)
			}
		}},
		{"kits sin resolver", "/catalog/kits?expand=false", 200, func(t *testing.T, b map[string]any) {
			if _, resolved := b["items"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)["product"]; resolved {
				t.Error("expand=false no debe resolver productos")
			}
		}},
		{"kits por perfil", "/catalog/kits?perfil=mama-primeriza", 200, func(t *testing.T, b map[string]any) {
			if b["total"] != 1.0 {
				t.Errorf("total = %v", b["total"])
			}
		}},
		{"búsqueda por texto en productos", "/catalog/products?q=se+me+cae+el+pelo", 200, func(t *testing.T, b map[string]any) {
			items := b["items"].([]any)
			if b["total"] != 2.0 || items[0].(map[string]any)["matchedTerms"] == nil {
				t.Errorf("resultado = %v", b)
			}
		}},
		{"búsqueda por texto en kits", "/catalog/kits?q=biotina", 200, func(t *testing.T, b map[string]any) {
			if b["total"] != 1.0 {
				t.Errorf("total = %v", b["total"])
			}
		}},
		{"q demasiado largo es 400", "/catalog/kits?q=" + strings.Repeat("a", 201), 400, nil},
		{"kit por slug", "/catalog/kits/kit-caida", 200, nil},
		{"kit inexistente es 404", "/catalog/kits/nope", 404, nil},
		{"perfil desconocido en kits es 400", "/catalog/kits?perfil=caida-del-cabello", 400, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body, header := get(t, router, tt.url)
			if status != tt.status {
				t.Fatalf("GET %s = %d; se esperaba %d (%v)", tt.url, status, tt.status, body)
			}
			if ct := header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			if tt.check != nil {
				tt.check(t, body)
			}
		})
	}

	t.Run("solo GET", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/catalog/products", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST = %d; se esperaba 405", rec.Code)
		}
	})
}

func TestCatalogLoadFailure(t *testing.T) {
	router := newCatalogRouter(stubLoader{err: errors.New("mongo caído")})
	status, body, _ := get(t, router, "/catalog/products")
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d; se esperaba 500", status)
	}
	// El mensaje al cliente es genérico: no filtra detalles internos.
	if body["error"] != "internal server error" {
		t.Errorf("error = %v", body["error"])
	}
}

func TestQueryList(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x?need=a,b&need=c&need=%20d%20,,", nil)
	got := queryList(r, "need")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("queryList = %v; se esperaba %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("queryList[%d] = %q; se esperaba %q", i, got[i], want[i])
		}
	}
}
