package catalog

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"¿Se te cae el pelo?", []string{"cae", "pelo"}},
		{"té verde", []string{"te", "verde"}}, // "té" (bebida) no es la palabra vacía "te"
		{"corredor de maratones", []string{"corredor", "maraton"}},
		{"dolor en las articulaciones", []string{"dolor", "articulacion"}},
		{"Champú", []string{"champu"}}, // sin equivalencias: se devuelve tal cual se escribió
		{"ojo ojos OJO", []string{"ojo"}},
		{"S1012", []string{"s1012"}},
		{"   ", nil},
		{"a", nil},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := Normalize(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Normalize(%q) = %v; se esperaba %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestEquivalences(t *testing.T) {
	tests := []struct{ word, want string }{
		{"pelo", "cabello"},
		{"cabello", "cabello"},
		{"cae", "caida"},
		{"caen", "caida"},
		{"champu", "shampoo"},
		{"gym", "gimnasio"},
		{"musculo", "muscular"},
		{"verde", "verde"},
	}
	for _, tt := range tests {
		if got := canon(tt.word); got != tt.want {
			t.Errorf("canon(%q) = %q; se esperaba %q", tt.word, got, tt.want)
		}
	}
}

func TestProductSearch(t *testing.T) {
	s := testSnapshot()

	tests := []struct {
		name  string
		f     ProductFilter
		want  []string // slugs en orden
		terms []string // matchedTerms del primero
	}{
		{"equivalencia pelo → cabello, por relevancia", ProductFilter{Query: "pelo", Limit: 50},
			[]string{"shampoo-bergamota", "gomitas-biotina"}, []string{"pelo"}},
		{"prefijo", ProductFilter{Query: "biot", Limit: 50}, []string{"gomitas-biotina"}, []string{"biot"}},
		{"clave de una variante", ProductFilter{Query: "s641", Limit: 50}, []string{"bb-cream"}, []string{"s641"}},
		{"texto y necesidad a la vez", ProductFilter{Query: "gomitas", Needs: []string{"caida-del-cabello"}, Limit: 50},
			[]string{"gomitas-biotina"}, []string{"gomita"}},
		{"sin coincidencias", ProductFilter{Query: "zzzz", Limit: 50}, []string{}, nil},
		{"sólo palabras vacías", ProductFilter{Query: "de la", Limit: 50}, []string{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := s.ProductList(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if got := slugs(items); !reflect.DeepEqual(got, tt.want) || total != len(tt.want) {
				t.Fatalf("resultado = %v (total %d); se esperaba %v", got, total, tt.want)
			}
			if len(items) == 0 {
				return
			}
			if got, _ := items[0]["matchedTerms"].([]string); !reflect.DeepEqual(got, tt.terms) {
				t.Errorf("matchedTerms = %v; se esperaba %v", got, tt.terms)
			}
			if _, ok := items[0]["score"]; !ok {
				t.Error("con q debe venir score")
			}
		})
	}

	t.Run("cada necesidad de la vista corta trae su texto contextual", func(t *testing.T) {
		docs := fixture()
		links := docs.Products[0]["needs"].(bson.A)
		links[0].(bson.M)["copy"] = bson.M{"micro": "Anticaída", "short": "Ayuda a fortalecer el cabello"}
		s := buildSnapshot(docs, time.Unix(0, 0))
		items, _, _ := s.ProductList(ProductFilter{Query: "bergamota", Limit: 50})
		needs := items[0]["needs"].([]any)
		first := needs[0].(map[string]any)
		if c, ok := first["copy"].(map[string]any); !ok || c["short"] != "Ayuda a fortalecer el cabello" {
			t.Errorf("needs[0] = %v", first)
		}
		if _, ok := needs[1].(map[string]any)["copy"]; ok {
			t.Error("una liga sin texto no debe traer copy")
		}
	})

	t.Run("la vista corta trae los textos de lámina", func(t *testing.T) {
		items, _, _ := s.ProductList(ProductFilter{Query: "bergamota", Limit: 50})
		c, ok := items[0]["copy"].(map[string]any)
		if !ok || c["micro"] != "micro shampoo-bergamota" {
			t.Errorf("copy = %v", items[0]["copy"])
		}
	})
}

func TestKitSearch(t *testing.T) {
	s := testSnapshot()

	tests := []struct {
		name  string
		q     string
		want  []string
		terms map[string][]string // matchedTerms por kit
	}{
		{"necesidad del kit con equivalencia", "se me cae el pelo", []string{"kit-caida-del-cabello"},
			map[string][]string{"kit-caida-del-cabello": {"cae", "pelo"}}},
		{"perfil", "corredor", []string{"kit-corredor"}, nil},
		{"producto dentro del kit", "glucosamina", []string{"kit-corredor"}, nil},
		{"varias palabras: cada kit trae sólo las suyas", "pelo creatina", []string{"kit-caida-del-cabello", "kit-corredor"},
			map[string][]string{"kit-caida-del-cabello": {"pelo"}, "kit-corredor": {"creatina"}}},
		{"sin coincidencias", "zzzz", []string{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := s.KitList(KitFilter{Query: tt.q, Expand: true, Limit: 50})
			if err != nil {
				t.Fatal(err)
			}
			if got := slugs(items); !reflect.DeepEqual(got, tt.want) || total != len(tt.want) {
				t.Fatalf("resultado = %v (total %d); se esperaba %v", got, total, tt.want)
			}
			for _, it := range items {
				want, ok := tt.terms[it["slug"].(string)]
				if !ok {
					continue
				}
				if got, _ := it["matchedTerms"].([]string); !reflect.DeepEqual(got, want) {
					t.Errorf("%s: matchedTerms = %v; se esperaba %v", it["slug"], got, want)
				}
			}
		})
	}

	t.Run("el título pesa más que un producto del kit", func(t *testing.T) {
		items, _, _ := s.KitList(KitFilter{Query: "cabello", Limit: 50})
		if len(items) == 0 || items[0]["slug"] != "kit-caida-del-cabello" {
			t.Errorf("primero = %v", slugs(items))
		}
	})
}
