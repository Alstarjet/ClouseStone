package catalog

import (
	"errors"
	"fmt"
	"sort"
)

// ErrNotFound indica que el producto o kit pedido no existe.
var ErrNotFound = errors.New("catalog: not found")

// ParamError indica un parámetro de consulta inválido (el handler lo traduce a 400).
type ParamError struct {
	Param  string
	Value  string
	Reason string
}

func (e *ParamError) Error() string {
	return fmt.Sprintf("invalid %s %q: %s", e.Param, e.Value, e.Reason)
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// page recorta una lista según skip y limit (limit <= 0 = sin límite).
func page[T any](xs []T, skip, limit int) []T {
	if skip < 0 {
		skip = 0
	}
	if skip >= len(xs) {
		return nil
	}
	xs = xs[skip:]
	if limit > 0 && limit < len(xs) {
		xs = xs[:limit]
	}
	return xs
}

// ---------- Vistas ----------

// productLite es la vista corta de un producto, pensada para listas y kits.
func productLite(p *product) map[string]any {
	r := p.raw
	out := map[string]any{"slug": p.slug, "key": p.key, "displayName": p.name}
	for _, k := range []string{"shortName", "rubros", "productType", "application", "regulatoryType", "audience", "cautions"} {
		if v, ok := r[k]; ok {
			out[k] = v
		}
	}

	image := ""
	variants := make([]any, 0, len(p.variants))
	for _, v := range p.variants {
		lv := map[string]any{"key": v["key"], "label": v["label"]}
		if img := str(v, "image"); img != "" {
			lv["image"] = img
			if image == "" {
				image = img
			}
		}
		variants = append(variants, lv)
	}
	out["variants"] = variants
	if image == "" {
		image = str(sub(sub(r, "research"), "imagen"), "url")
	}
	out["image"] = image

	// Textos de lámina completos (micro, short y solo) e ingredientes: con esto el
	// cliente muestra el producto solo o dentro de un kit sin pedir el detalle.
	if c := sub(r, "copy"); c != nil {
		out["copy"] = c
	}
	if ings, ok := r["ingredients"]; ok {
		out["ingredients"] = ings
	}
	needs := make([]any, 0, len(p.links))
	for _, l := range p.links {
		needs = append(needs, map[string]any{"need": l.need, "relevance": l.relevance, "role": l.role})
	}
	out["needs"] = needs
	return out
}

func variantByKey(p *product, key string) map[string]any {
	for _, v := range p.variants {
		if str(v, "key") == key {
			return v
		}
	}
	return nil
}

// kitView devuelve el kit tal cual está guardado y, si expand, con cada item
// resuelto contra los productos por su clave (product + variant).
func (s *Snapshot) kitView(k *kit, expand bool) map[string]any {
	out := cloneMap(k.raw)
	if !expand {
		return out
	}
	sorted := append([]kitItem(nil), k.items...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].order < sorted[j].order })
	items := make([]any, 0, len(sorted))
	for _, it := range sorted {
		item := cloneMap(it.raw)
		if p, ok := s.productByKey[it.key]; ok {
			item["product"] = productLite(p)
			item["variant"] = variantByKey(p, it.key)
		} else {
			item["product"] = nil
			item["missing"] = true
		}
		items = append(items, item)
	}
	out["items"] = items
	return out
}

// ---------- Productos ----------

// ProductFilter son los filtros del listado de productos.
type ProductFilter struct {
	Query        string   // texto libre: nombre, clave, tipo, necesidades, ingredientes y textos
	Needs        []string // slugs de necesidades: se listan los productos ligados a ellas
	Rubro        string
	Type         string // productType
	Application  string // topico | oral | ambiental | hogar
	MinRelevance float64
	Limit, Skip  int
}

// ProductList devuelve la página pedida y el total de coincidencias. Con need,
// la lista es exhaustiva (todos los productos ligados) y se ordena por puntaje =
// Σ relevancia de las ligas que coinciden; con q, sólo los que coinciden con el
// texto, y su puntaje se suma. Sin need ni q, por nombre.
func (s *Snapshot) ProductList(f ProductFilter) (items []map[string]any, total int, err error) {
	for _, slug := range f.Needs {
		if _, ok := s.knownNeeds[slug]; !ok {
			return nil, 0, &ParamError{Param: "need", Value: slug, Reason: "unknown need"}
		}
	}
	words, searching := queryWords(f.Query)
	if searching && len(words) == 0 {
		return []map[string]any{}, 0, nil // sólo palabras vacías: nada que buscar
	}
	ranked := len(f.Needs) > 0 || searching

	type hit struct {
		p       *product
		score   float64
		matched []needLink
		terms   []string
	}
	var hits []hit
	for _, p := range s.products {
		if f.Rubro != "" && !contains(p.rubros, f.Rubro) {
			continue
		}
		if f.Type != "" && p.productType != f.Type {
			continue
		}
		if f.Application != "" && p.application != f.Application {
			continue
		}
		h := hit{p: p}
		if len(f.Needs) > 0 {
			for _, l := range p.links {
				if contains(f.Needs, l.need) && l.relevance >= f.MinRelevance {
					h.score += l.relevance
					h.matched = append(h.matched, l)
				}
			}
			if len(h.matched) == 0 {
				continue
			}
		}
		if searching {
			text, matched := p.terms.score(words)
			if len(matched) == 0 {
				continue
			}
			h.score += text
			h.terms = matched
		}
		hits = append(hits, h)
	}
	if ranked {
		sort.SliceStable(hits, func(i, j int) bool {
			if hits[i].score != hits[j].score {
				return hits[i].score > hits[j].score
			}
			return sortKey(hits[i].p.name) < sortKey(hits[j].p.name)
		})
	}

	total = len(hits)
	items = []map[string]any{}
	for _, h := range page(hits, f.Skip, f.Limit) {
		item := productLite(h.p)
		if ranked {
			item["score"] = round2(h.score)
		}
		if len(f.Needs) > 0 {
			matched := make([]any, 0, len(h.matched))
			for _, l := range h.matched {
				matched = append(matched, map[string]any{"need": l.need, "relevance": l.relevance, "role": l.role})
			}
			item["matched"] = matched
		}
		if searching {
			item["matchedTerms"] = h.terms
		}
		items = append(items, item)
	}
	return items, total, nil
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// queryWords normaliza el texto de búsqueda. searching indica si se pidió una
// búsqueda (q no vacío), aunque todas sus palabras resulten vacías.
func queryWords(q string) (words []string, searching bool) {
	if q == "" {
		return nil, false
	}
	return Normalize(q), true
}

// Product devuelve el producto completo por clave del catálogo (cualquier
// variante) o por slug, más los kits en los que aparece.
func (s *Snapshot) Product(id string) (map[string]any, error) {
	p, ok := s.productByKey[id]
	if !ok {
		p, ok = s.productBySlug[id]
	}
	if !ok {
		return nil, ErrNotFound
	}

	out := cloneMap(p.raw)
	if v := variantByKey(p, id); v != nil {
		out["variant"] = v
	}

	keys := map[string]struct{}{}
	for _, v := range p.variants {
		keys[str(v, "key")] = struct{}{}
	}
	kits := []any{}
	for _, k := range s.kits {
		for _, it := range k.items {
			if _, in := keys[it.key]; in {
				kits = append(kits, map[string]any{"slug": k.slug, "title": k.title})
				break
			}
		}
	}
	out["kits"] = kits
	return out, nil
}

// ---------- Kits ----------

// KitFilter son los filtros del listado de kits.
type KitFilter struct {
	Query       string   // texto libre: título, necesidades, perfiles y productos del kit
	Needs       []string // el kit responde a alguna de estas necesidades
	Perfiles    []string // el kit responde a alguno de estos perfiles
	Expand      bool     // resolver los productos de cada item
	Limit, Skip int
}

// KitList devuelve los kits curados que coinciden (sin filtros, todos), ordenados
// por título; con q, sólo los que coinciden con el texto, del más al menos
// relevante. Cada kit lleva 3 o 4 productos, aunque haya más relacionados.
func (s *Snapshot) KitList(f KitFilter) (items []map[string]any, total int, err error) {
	for _, slug := range f.Needs {
		if _, ok := s.knownNeeds[slug]; !ok {
			return nil, 0, &ParamError{Param: "need", Value: slug, Reason: "unknown need"}
		}
	}
	for _, slug := range f.Perfiles {
		if _, ok := s.knownPerfiles[slug]; !ok {
			return nil, 0, &ParamError{Param: "perfil", Value: slug, Reason: "unknown perfil"}
		}
	}

	words, searching := queryWords(f.Query)
	if searching && len(words) == 0 {
		return []map[string]any{}, 0, nil // sólo palabras vacías: nada que buscar
	}

	type hit struct {
		k     *kit
		score float64
		terms []string
	}
	var hits []hit
	for _, k := range s.kits {
		if len(f.Needs)+len(f.Perfiles) > 0 {
			match := false
			for _, n := range f.Needs {
				match = match || contains(k.needs, n)
			}
			for _, p := range f.Perfiles {
				match = match || contains(k.perfiles, p)
			}
			if !match {
				continue
			}
		}
		h := hit{k: k}
		if searching {
			if h.score, h.terms = k.terms.score(words); len(h.terms) == 0 {
				continue
			}
		}
		hits = append(hits, h)
	}
	if searching {
		sort.SliceStable(hits, func(i, j int) bool {
			if hits[i].score != hits[j].score {
				return hits[i].score > hits[j].score
			}
			return sortKey(hits[i].k.title) < sortKey(hits[j].k.title)
		})
	}

	total = len(hits)
	items = []map[string]any{}
	for _, h := range page(hits, f.Skip, f.Limit) {
		view := s.kitView(h.k, f.Expand)
		if searching {
			view["score"] = round2(h.score)
			view["matchedTerms"] = h.terms
		}
		items = append(items, view)
	}
	return items, total, nil
}

// Kit devuelve un kit por slug con sus productos resueltos.
func (s *Snapshot) Kit(slug string) (map[string]any, error) {
	k, ok := s.kitBySlug[slug]
	if !ok {
		return nil, ErrNotFound
	}
	return s.kitView(k, true), nil
}
