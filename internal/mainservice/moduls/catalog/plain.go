// Package catalog implementa las consultas de solo lectura del catálogo curado
// de productos y kits (colecciones products_info y product_kits).
//
// Como el catálogo es pequeño (cientos de documentos) y casi no cambia, el
// servicio lo carga completo a un snapshot en memoria con vencimiento (TTL). Todo
// el filtrado, ordenamiento y resolución de kits se hace sobre ese snapshot con
// funciones puras, fáciles de probar sin Mongo.
//
// Identidad: nada depende del _id de Mongo. Los productos se identifican por su
// clave del catálogo (key / variants[].key) y por slug; los kits, por slug, y
// referencian productos por clave.
package catalog

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// plain convierte valores BSON a tipos simples de Go (map[string]any, []any,
// string, números) para poder navegarlos y serializarlos a JSON sin sorpresas.
func plain(v any) any {
	switch t := v.(type) {
	case bson.M:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = plain(val)
		}
		return m
	case bson.D:
		m := make(map[string]any, len(t))
		for _, e := range t {
			m[e.Key] = plain(e.Value)
		}
		return m
	case bson.A:
		a := make([]any, len(t))
		for i, val := range t {
			a[i] = plain(val)
		}
		return a
	case primitive.ObjectID:
		return t.Hex()
	case primitive.DateTime:
		return t.Time().UTC().Format(time.RFC3339)
	default:
		return v
	}
}

// plainDoc convierte un documento y descarta su _id: el _id es de Mongo y el
// API no debe depender de él (las referencias usan key y slug).
func plainDoc(d bson.M) map[string]any {
	m, _ := plain(d).(map[string]any)
	delete(m, "_id")
	return m
}

// cloneMap copia superficialmente un mapa para poder añadirle claves sin
// modificar el snapshot compartido.
func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+4)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func strs(m map[string]any, k string) []string {
	a, _ := m[k].([]any)
	out := make([]string, 0, len(a))
	for _, x := range a {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func maps(m map[string]any, k string) []map[string]any {
	a, _ := m[k].([]any)
	out := make([]map[string]any, 0, len(a))
	for _, x := range a {
		if mm, ok := x.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

func sub(m map[string]any, k string) map[string]any {
	mm, _ := m[k].(map[string]any)
	return mm
}

func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}
