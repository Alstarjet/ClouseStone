package catalog

import (
	"context"
	"errors"
	"financial-Assistant/internal/mainservice/models"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fixture arma un catálogo pequeño con la misma forma que devuelve el driver de
// Mongo (bson.M / bson.A y un _id que debe descartarse).
func fixture() models.CatalogDocs {
	link := func(need string, rel float64, role string) bson.M {
		return bson.M{"need": need, "relevance": rel, "role": role}
	}
	variant := func(key, label, image string) bson.M {
		return bson.M{"key": key, "name": "NOMBRE " + key, "label": label, "image": image}
	}
	product := func(slug, key, name, ptype string, rubros []string, variants bson.A, links bson.A) bson.M {
		r := bson.A{}
		for _, x := range rubros {
			r = append(r, x)
		}
		return bson.M{"_id": primitive.NewObjectID(), "slug": slug, "key": key, "displayName": name, "shortName": name,
			"productType": ptype, "application": "topico", "rubros": r, "variants": variants, "needs": links,
			"copy":     bson.M{"micro": "micro " + slug, "short": "short " + slug},
			"research": bson.M{"confianza": "media", "imagen": bson.M{"url": "http://research/" + slug}}}
	}

	return models.CatalogDocs{
		Products: []bson.M{
			product("shampoo-bergamota", "S1012", "Shampoo de Bergamota", "shampoo", []string{"belleza"},
				bson.A{variant("S1012", "", "http://img/S1012")},
				bson.A{link("caida-del-cabello", 1.0, "principal"), link("cuidado-del-cabello", 0.9, "complemento")}),
			product("gomitas-biotina", "S1202", "Gomitas con Biotina", "gomitas", []string{"belleza", "salud"},
				bson.A{variant("S1202", "", "")}, bson.A{link("caida-del-cabello", 0.7, "complemento")}),
			product("creatina-monohidratada", "S1418", "Creatina Monohidratada", "suplemento-en-polvo", []string{"fitness"},
				bson.A{variant("S1418", "", "http://img/S1418")},
				bson.A{link("masa-muscular", 1.0, "principal"), link("energia-y-vitalidad", 0.5, "complemento")}),
			product("glucosamina", "S622", "Glucosamina", "suplemento-en-polvo", []string{"salud", "fitness"},
				bson.A{variant("S622", "", "http://img/S622")}, bson.A{link("salud-articular", 1.0, "principal")}),
			product("bb-cream", "S640", "BB Cream", "bb-cream", []string{"belleza"},
				bson.A{variant("S640", "Light", "http://img/S640"), variant("S641", "Medium", "http://img/S641")}, bson.A{}),
		},
		Kits: []bson.M{
			{"_id": primitive.NewObjectID(), "slug": "kit-caida-del-cabello", "title": "Recupera tu cabello", "needs": bson.A{"caida-del-cabello"}, "perfiles": bson.A{},
				"items": bson.A{
					bson.M{"key": "S1202", "order": 2, "role": "complemento"},
					bson.M{"key": "S1012", "order": 1, "role": "principal"},
					bson.M{"key": "S9999", "order": 3, "role": "complemento"}, // clave que ya no existe
				}},
			{"_id": primitive.NewObjectID(), "slug": "kit-corredor", "title": "Kit del corredor", "needs": bson.A{"energia-y-vitalidad"}, "perfiles": bson.A{"corredor"},
				"items": bson.A{
					bson.M{"key": "S1418", "order": 1, "role": "principal"},
					bson.M{"key": "S622", "order": 2, "role": "complemento"},
					bson.M{"key": "S641", "order": 3, "role": "complemento"}, // variante secundaria de BB Cream
				}},
		},
	}
}

func testSnapshot() *Snapshot { return buildSnapshot(fixture(), time.Unix(0, 0)) }

func slugs(items []map[string]any) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it["slug"].(string))
	}
	return out
}

func TestSnapshotIndexing(t *testing.T) {
	s := testSnapshot()

	if got := s.Counts(); got["products"] != 5 || got["kits"] != 2 {
		t.Errorf("conteos inesperados: %v", got)
	}
	if s.productByKey["S641"] == nil || s.productByKey["S641"].slug != "bb-cream" {
		t.Error("una variante secundaria (S641) debe resolver a su producto")
	}
	for _, p := range s.products {
		if _, ok := p.raw["_id"]; ok {
			t.Errorf("%s: el _id de Mongo debe descartarse", p.slug)
		}
	}
	// Orden de listado: por nombre sin acentos ni mayúsculas.
	var names []string
	for _, p := range s.products {
		names = append(names, p.name)
	}
	want := []string{"BB Cream", "Creatina Monohidratada", "Glucosamina", "Gomitas con Biotina", "Shampoo de Bergamota"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("orden de productos = %v; se esperaba %v", names, want)
	}
}

func TestProductList(t *testing.T) {
	s := testSnapshot()

	t.Run("por necesidad: exhaustivo y ordenado por relevancia", func(t *testing.T) {
		items, total, err := s.ProductList(ProductFilter{Needs: []string{"caida-del-cabello"}, Limit: 50})
		if err != nil || total != 2 {
			t.Fatalf("total = %d, err = %v", total, err)
		}
		if got := slugs(items); !reflect.DeepEqual(got, []string{"shampoo-bergamota", "gomitas-biotina"}) {
			t.Errorf("orden = %v", got)
		}
		if items[0]["score"].(float64) != 1 || items[1]["score"].(float64) != 0.7 {
			t.Errorf("scores = %v, %v", items[0]["score"], items[1]["score"])
		}
		if items[0]["image"] != "http://img/S1012" {
			t.Errorf("image = %v", items[0]["image"])
		}
	})

	t.Run("varias necesidades suman su relevancia", func(t *testing.T) {
		items, _, err := s.ProductList(ProductFilter{Needs: []string{"caida-del-cabello", "cuidado-del-cabello"}, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		if items[0]["slug"] != "shampoo-bergamota" || items[0]["score"].(float64) != 1.9 {
			t.Errorf("primero = %v (score %v)", items[0]["slug"], items[0]["score"])
		}
	})

	t.Run("sin imagen en la variante usa la de research", func(t *testing.T) {
		items, _, _ := s.ProductList(ProductFilter{Type: "gomitas", Limit: 50})
		if len(items) != 1 || items[0]["image"] != "http://research/gomitas-biotina" {
			t.Errorf("image = %v", items[0]["image"])
		}
	})

	t.Run("filtros de rubro, tipo y aplicación", func(t *testing.T) {
		items, total, _ := s.ProductList(ProductFilter{Rubro: "fitness", Limit: 50})
		if total != 2 {
			t.Errorf("rubro fitness: total = %d (%v)", total, slugs(items))
		}
		if _, total, _ = s.ProductList(ProductFilter{Application: "oral", Limit: 50}); total != 0 {
			t.Errorf("application oral: total = %d", total)
		}
	})

	t.Run("min_relevance descarta ligas débiles", func(t *testing.T) {
		_, total, _ := s.ProductList(ProductFilter{Needs: []string{"caida-del-cabello"}, MinRelevance: 0.8, Limit: 50})
		if total != 1 {
			t.Errorf("total = %d; se esperaba 1", total)
		}
	})

	t.Run("sin filtros: todos, por nombre, con paginación", func(t *testing.T) {
		items, total, _ := s.ProductList(ProductFilter{Limit: 2, Skip: 1})
		if total != 5 || len(items) != 2 {
			t.Fatalf("total = %d, len = %d", total, len(items))
		}
		if got := slugs(items); !reflect.DeepEqual(got, []string{"creatina-monohidratada", "glucosamina"}) {
			t.Errorf("página = %v", got)
		}
		if _, ok := items[0]["score"]; ok {
			t.Error("sin need no debe haber score")
		}
		if items, total, _ = s.ProductList(ProductFilter{Limit: 10, Skip: 99}); total != 5 || len(items) != 0 {
			t.Errorf("skip fuera de rango: total = %d, len = %d", total, len(items))
		}
	})

	t.Run("una necesidad que no existe en el catálogo es error", func(t *testing.T) {
		var perr *ParamError
		if _, _, err := s.ProductList(ProductFilter{Needs: []string{"inventada"}}); !errors.As(err, &perr) || perr.Param != "need" {
			t.Errorf("err = %v", err)
		}
	})
}

func TestProduct(t *testing.T) {
	s := testSnapshot()

	t.Run("por clave de una variante secundaria", func(t *testing.T) {
		doc, err := s.Product("S641")
		if err != nil {
			t.Fatal(err)
		}
		if doc["slug"] != "bb-cream" || doc["variant"].(map[string]any)["label"] != "Medium" {
			t.Errorf("doc = %v", doc["variant"])
		}
		kits := doc["kits"].([]any)
		if len(kits) != 1 || kits[0].(map[string]any)["slug"] != "kit-corredor" {
			t.Errorf("kits del producto = %v", kits)
		}
	})

	t.Run("por slug no hay variante seleccionada", func(t *testing.T) {
		doc, err := s.Product("shampoo-bergamota")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := doc["variant"]; ok {
			t.Error("buscando por slug no hay variante seleccionada")
		}
		if doc["kits"].([]any)[0].(map[string]any)["slug"] != "kit-caida-del-cabello" {
			t.Errorf("kits = %v", doc["kits"])
		}
	})

	t.Run("no modifica el snapshot compartido", func(t *testing.T) {
		if _, err := s.Product("S1012"); err != nil {
			t.Fatal(err)
		}
		if _, polluted := s.productBySlug["shampoo-bergamota"].raw["kits"]; polluted {
			t.Error("Product no debe añadir kits al documento del snapshot")
		}
	})

	if _, err := s.Product("S0000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("inexistente: err = %v", err)
	}
}

func TestKits(t *testing.T) {
	s := testSnapshot()

	t.Run("lista con items resueltos y en su orden", func(t *testing.T) {
		items, total, err := s.KitList(KitFilter{Expand: true, Limit: 50})
		if err != nil || total != 2 {
			t.Fatalf("total = %d, err = %v", total, err)
		}
		// Orden por título: "Kit del corredor" < "Recupera tu cabello"
		if got := slugs(items); !reflect.DeepEqual(got, []string{"kit-corredor", "kit-caida-del-cabello"}) {
			t.Errorf("orden de kits = %v", got)
		}
		kit := items[1]["items"].([]any)
		if kit[0].(map[string]any)["key"] != "S1012" || kit[1].(map[string]any)["key"] != "S1202" {
			t.Errorf("los items deben ir por order: %v", kit)
		}
		if p := kit[0].(map[string]any)["product"].(map[string]any); p["slug"] != "shampoo-bergamota" {
			t.Errorf("product = %v", p["slug"])
		}
	})

	t.Run("una clave inexistente se marca como missing sin romper el kit", func(t *testing.T) {
		doc, err := s.Kit("kit-caida-del-cabello")
		if err != nil {
			t.Fatal(err)
		}
		last := doc["items"].([]any)[2].(map[string]any)
		if last["missing"] != true || last["product"] != nil {
			t.Errorf("item = %v", last)
		}
	})

	t.Run("la clave de una variante muestra esa variante", func(t *testing.T) {
		doc, _ := s.Kit("kit-corredor")
		it := doc["items"].([]any)[2].(map[string]any)
		if it["variant"].(map[string]any)["label"] != "Medium" {
			t.Errorf("variant = %v", it["variant"])
		}
		if it["product"].(map[string]any)["slug"] != "bb-cream" {
			t.Errorf("product = %v", it["product"])
		}
	})

	t.Run("filtros por necesidad y por perfil; expand=false", func(t *testing.T) {
		items, total, _ := s.KitList(KitFilter{Needs: []string{"caida-del-cabello"}, Limit: 50})
		if total != 1 || items[0]["slug"] != "kit-caida-del-cabello" {
			t.Errorf("por need: %v", slugs(items))
		}
		if _, ok := items[0]["items"].([]any)[0].(map[string]any)["product"]; ok {
			t.Error("con expand=false los items no se resuelven")
		}
		items, total, _ = s.KitList(KitFilter{Perfiles: []string{"corredor"}, Expand: true, Limit: 50})
		if total != 1 || items[0]["slug"] != "kit-corredor" {
			t.Errorf("por perfil: %v", slugs(items))
		}
	})

	t.Run("errores", func(t *testing.T) {
		var perr *ParamError
		if _, _, err := s.KitList(KitFilter{Perfiles: []string{"nadie"}}); !errors.As(err, &perr) || perr.Param != "perfil" {
			t.Errorf("perfil desconocido: err = %v", err)
		}
		if _, _, err := s.KitList(KitFilter{Needs: []string{"nada"}}); !errors.As(err, &perr) || perr.Param != "need" {
			t.Errorf("need desconocida: err = %v", err)
		}
		if _, err := s.Kit("nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

// fakeLoader cuenta las cargas y permite simular fallos de Mongo.
type fakeLoader struct {
	calls int
	err   error
}

func (f *fakeLoader) LoadCatalogDocs(context.Context) (models.CatalogDocs, error) {
	f.calls++
	if f.err != nil {
		return models.CatalogDocs{}, f.err
	}
	return fixture(), nil
}

func TestServiceCache(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	newSvc := func(l *fakeLoader) *Service {
		s := NewService(l, time.Minute)
		s.now = func() time.Time { return now }
		return s
	}

	t.Run("dentro del TTL se reutiliza el snapshot", func(t *testing.T) {
		l := &fakeLoader{}
		s := newSvc(l)
		first, err := s.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		now = now.Add(30 * time.Second)
		second, _ := s.Snapshot(ctx)
		if l.calls != 1 || first != second {
			t.Errorf("cargas = %d; se esperaba 1 y el mismo snapshot", l.calls)
		}
	})

	t.Run("al vencer el TTL se recarga", func(t *testing.T) {
		l := &fakeLoader{}
		s := newSvc(l)
		_, _ = s.Snapshot(ctx)
		now = now.Add(2 * time.Minute)
		if _, err := s.Snapshot(ctx); err != nil || l.calls != 2 {
			t.Errorf("cargas = %d, err = %v", l.calls, err)
		}
	})

	t.Run("si la recarga falla se sirve la copia anterior y se espera antes de reintentar", func(t *testing.T) {
		l := &fakeLoader{}
		s := newSvc(l)
		_, _ = s.Snapshot(ctx)
		l.err = errors.New("mongo caído")
		now = now.Add(2 * time.Minute)
		snap, err := s.Snapshot(ctx)
		if err != nil || snap == nil {
			t.Fatalf("debe servir la copia anterior: err = %v", err)
		}
		calls := l.calls
		_, _ = s.Snapshot(ctx) // dentro del backoff: no vuelve a cargar
		if l.calls != calls {
			t.Errorf("no debía reintentar dentro del backoff (cargas %d → %d)", calls, l.calls)
		}
		now = now.Add(retryBackoff + time.Second)
		l.err = nil
		if _, err := s.Snapshot(ctx); err != nil || l.calls != calls+1 {
			t.Errorf("tras el backoff debe reintentar: cargas = %d, err = %v", l.calls, err)
		}
	})

	t.Run("sin copia previa, el error se propaga", func(t *testing.T) {
		s := newSvc(&fakeLoader{err: errors.New("mongo caído")})
		if _, err := s.Snapshot(ctx); err == nil {
			t.Error("se esperaba un error")
		}
	})
}
