package catalog

import "strings"

// Búsqueda por texto libre (?q=) sobre productos y kits. El índice de términos
// se arma en memoria al construir el snapshot; no hay campos precalculados en
// Mongo.

// stopwords que no aportan a la búsqueda. Se comparan ANTES de quitar acentos,
// para que "té" (la bebida) no se confunda con "te" (el pronombre).
var stopwords = map[string]struct{}{}

// equivalences son palabras que el catálogo escribe de otra forma: la búsqueda
// las trata como la misma (pelo → cabello). Lista corta y deliberada.
var equivalences = [][]string{
	{"cabello", "pelo", "cabellera"},
	{"caída", "cae", "caen", "caer"}, // "se me cae el pelo" → caída del cabello
	{"shampoo", "champú", "shampú"},
	{"gimnasio", "gym"},
	{"muscular", "músculo"},
	{"articular", "articulación", "coyuntura"},
	{"granito", "grano"},
}

// canonical asocia cada palabra normalizada de equivalences con la de su grupo.
var canonical = map[string]string{}

func init() {
	for _, w := range strings.Fields("de del la las el los lo un una unos unas y e o u a al en con para por sin " +
		"mi mis tu tus su sus se me te nos le les que qué muy mas más es son soy estoy ya " +
		"quiero tengo busco necesito algo alguno alguna como cómo hay voy hago hacer") {
		stopwords[w] = struct{}{}
	}
	for _, group := range equivalences {
		head := singular(foldAccents(group[0]))
		for _, w := range group {
			canonical[singular(foldAccents(w))] = head
		}
	}
}

func foldAccents(s string) string { return accentFolder.Replace(s) }

func isVowel(b byte) bool { return strings.IndexByte("aeiou", b) >= 0 }

// singular aplica un singular "ligero" (articulaciones → articulacion, ojos →
// ojo). No es lingüísticamente perfecto ("diabetes" → "diabet"); no importa,
// porque se aplica igual al indexar y al buscar.
func singular(t string) string {
	n := len(t)
	switch {
	case n > 4 && strings.HasSuffix(t, "es") && !isVowel(t[n-3]):
		return t[:n-2]
	case n > 3 && strings.HasSuffix(t, "s"):
		return t[:n-1]
	default:
		return t
	}
}

// Normalize convierte un texto en palabras comparables: minúsculas, sólo letras
// y dígitos, sin palabras vacías, sin acentos, en singular ligero y sin
// repetidos. El frontend aplica la misma regla a lo que guarda en el teléfono.
func Normalize(text string) []string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', strings.ContainsRune("áéíóúüñ", r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	seen := map[string]struct{}{}
	var out []string
	for _, t := range strings.Fields(b.String()) {
		if len([]rune(t)) <= 1 {
			continue
		}
		if _, stop := stopwords[t]; stop {
			continue
		}
		t = singular(foldAccents(t))
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func canon(t string) string {
	if c, ok := canonical[t]; ok {
		return c
	}
	return t
}

// minPrefix es el largo mínimo para que una palabra de la búsqueda coincida como
// inicio de un término ("biot" → "biotina").
const minPrefix = 3

// terms es el índice de un documento: término canónico → peso del campo de
// mayor peso donde aparece.
type terms map[string]float64

func (ix terms) add(text string, weight float64) {
	for _, t := range Normalize(text) {
		c := canon(t)
		if weight > ix[c] {
			ix[c] = weight
		}
	}
}

// slugText convierte un slug en texto ("caida-del-cabello" → "caida del cabello").
func slugText(s string) string { return strings.ReplaceAll(s, "-", " ") }

// match devuelve el peso con que una palabra canónica de la búsqueda coincide
// con el documento: igual a un término o, si tiene al menos minPrefix letras,
// como inicio de uno.
func (ix terms) match(word string) float64 {
	best := ix[word]
	if len(word) >= minPrefix {
		for term, w := range ix {
			if w > best && strings.HasPrefix(term, word) {
				best = w
			}
		}
	}
	return best
}

// score suma, por cada palabra de la búsqueda, el peso con que coincide, y
// devuelve las palabras que coincidieron tal como las normalizó Normalize (sin
// aplicar equivalencias), para que el cliente las guarde como palabras
// compatibles del documento.
func (ix terms) score(words []string) (total float64, matched []string) {
	for _, w := range words {
		if s := ix.match(canon(w)); s > 0 {
			total += s
			matched = append(matched, w)
		}
	}
	return total, matched
}

// Pesos por campo: lo que nombra al documento pesa más que su texto de apoyo.
const (
	weightKey     = 3.0 // clave del catálogo (S1012)
	weightName    = 2.0 // nombre del producto; título, subtítulo, necesidades y perfiles del kit
	weightNeed    = 1.5 // necesidades del producto (× relevancia) y su tipo
	weightRelated = 1.0 // ingredientes, variantes; productos dentro de un kit
	weightText    = 0.5 // textos de lámina, rubros
)

func productTerms(p *product) terms {
	ix := terms{}
	r := p.raw
	ix.add(p.key, weightKey)
	ix.add(p.name, weightName)
	ix.add(str(r, "shortName"), weightName)
	ix.add(slugText(p.productType), weightNeed)
	for _, l := range p.links {
		ix.add(slugText(l.need), max(weightText, weightNeed*l.relevance))
	}
	for _, v := range p.variants {
		ix.add(str(v, "key"), weightKey)
		ix.add(str(v, "name"), weightRelated)
		ix.add(str(v, "label"), weightRelated)
	}
	for _, ing := range maps(r, "ingredients") {
		ix.add(slugText(str(ing, "ingredient")), weightRelated)
	}
	for _, rubro := range p.rubros {
		ix.add(slugText(rubro), weightText)
	}
	addCopy(ix, sub(r, "copy"))
	for _, l := range maps(r, "needs") {
		addCopy(ix, sub(l, "copy"))
	}
	return ix
}

// kitTerms indexa el kit y, con menor peso, los productos que contiene.
func kitTerms(k *kit, productByKey map[string]*product) terms {
	ix := terms{}
	ix.add(k.title, weightName)
	ix.add(str(k.raw, "subtitle"), weightName)
	for _, n := range k.needs {
		ix.add(slugText(n), weightName)
	}
	for _, p := range k.perfiles {
		ix.add(slugText(p), weightName)
	}
	for _, it := range k.items {
		ix.add(it.key, weightRelated)
		addCopy(ix, sub(it.raw, "copy"))
		p, ok := productByKey[it.key]
		if !ok {
			continue
		}
		ix.add(p.name, weightRelated)
		ix.add(str(p.raw, "shortName"), weightRelated)
		ix.add(slugText(p.productType), weightText)
		for _, l := range p.links {
			ix.add(slugText(l.need), weightText)
		}
	}
	return ix
}

func addCopy(ix terms, c map[string]any) {
	if c == nil {
		return
	}
	ix.add(str(c, "micro"), weightText)
	ix.add(str(c, "short"), weightText)
	if solo := sub(c, "solo"); solo != nil {
		ix.add(str(solo, "headline"), weightText)
		ix.add(str(solo, "howToUse"), weightText)
		for _, f := range strs(solo, "fragments") {
			ix.add(f, weightText)
		}
	}
}
