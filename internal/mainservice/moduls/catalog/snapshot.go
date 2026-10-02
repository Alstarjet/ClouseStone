package catalog

import (
	"financial-Assistant/internal/mainservice/models"
	"sort"
	"strings"
	"time"
)

// Snapshot es una copia inmutable del catálogo en memoria. Se construye una vez
// por carga y se comparte entre peticiones; nunca se modifica después.
type Snapshot struct {
	// LoadedAt es el momento en que se cargó desde Mongo.
	LoadedAt time.Time

	products      []*product
	productByKey  map[string]*product // por cualquier variants[].key
	productBySlug map[string]*product
	kits          []*kit
	kitBySlug     map[string]*kit

	// Slugs de necesidades y perfiles que aparecen en los productos y kits; sirven
	// para rechazar con 400 un filtro con un slug que no existe en el catálogo.
	knownNeeds    map[string]struct{}
	knownPerfiles map[string]struct{}
}

type needLink struct {
	need      string
	relevance float64
	role      string
}

type product struct {
	raw         map[string]any
	slug        string
	key         string
	name        string
	productType string
	application string
	rubros      []string
	variants    []map[string]any
	links       []needLink
	terms       terms // índice para la búsqueda por texto
}

type kitItem struct {
	raw   map[string]any
	key   string
	order int
}

type kit struct {
	raw      map[string]any
	slug     string
	title    string
	needs    []string
	perfiles []string
	items    []kitItem
	terms    terms // índice para la búsqueda por texto
}

var accentFolder = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")

// sortKey es la clave de ordenamiento alfabético: minúsculas y sin acentos.
func sortKey(s string) string { return foldAccents(strings.ToLower(s)) }

// buildSnapshot indexa los documentos crudos. Los documentos sin slug se omiten.
func buildSnapshot(docs models.CatalogDocs, loadedAt time.Time) *Snapshot {
	s := &Snapshot{
		LoadedAt:      loadedAt,
		productByKey:  map[string]*product{},
		productBySlug: map[string]*product{},
		kitBySlug:     map[string]*kit{},
		knownNeeds:    map[string]struct{}{},
		knownPerfiles: map[string]struct{}{},
	}

	for _, d := range docs.Products {
		m := plainDoc(d)
		p := &product{
			raw: m, slug: str(m, "slug"), key: str(m, "key"), name: str(m, "displayName"),
			productType: str(m, "productType"), application: str(m, "application"),
			rubros: strs(m, "rubros"), variants: maps(m, "variants"),
		}
		if p.slug == "" {
			continue
		}
		for _, l := range maps(m, "needs") {
			link := needLink{need: str(l, "need"), relevance: num(l["relevance"]), role: str(l, "role")}
			p.links = append(p.links, link)
			s.knownNeeds[link.need] = struct{}{}
		}
		s.products = append(s.products, p)
		s.productBySlug[p.slug] = p
		for _, v := range p.variants {
			if k := str(v, "key"); k != "" {
				s.productByKey[k] = p
			}
		}
		if p.key != "" {
			s.productByKey[p.key] = p
		}
		p.terms = productTerms(p)
	}

	for _, d := range docs.Kits {
		m := plainDoc(d)
		k := &kit{raw: m, slug: str(m, "slug"), title: str(m, "title"), needs: strs(m, "needs"), perfiles: strs(m, "perfiles")}
		if k.slug == "" {
			continue
		}
		for _, n := range k.needs {
			s.knownNeeds[n] = struct{}{}
		}
		for _, p := range k.perfiles {
			s.knownPerfiles[p] = struct{}{}
		}
		for _, it := range maps(m, "items") {
			k.items = append(k.items, kitItem{raw: it, key: str(it, "key"), order: int(num(it["order"]))})
		}
		k.terms = kitTerms(k, s.productByKey)
		s.kits = append(s.kits, k)
		s.kitBySlug[k.slug] = k
	}

	// Orden estable de listado: por nombre sin acentos ni mayúsculas.
	sort.SliceStable(s.products, func(i, j int) bool { return sortKey(s.products[i].name) < sortKey(s.products[j].name) })
	sort.SliceStable(s.kits, func(i, j int) bool { return sortKey(s.kits[i].title) < sortKey(s.kits[j].title) })
	return s
}

// Counts devuelve el número de documentos cargados (útil para diagnóstico).
func (s *Snapshot) Counts() map[string]int {
	return map[string]int{"products": len(s.products), "kits": len(s.kits)}
}
