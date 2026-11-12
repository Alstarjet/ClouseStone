package models

// ProductCartItem es un renglón de producto dentro de un cargo o de un pedido.
// Lleva los mismos campos que el renglón que arma la app (productCartItem en
// src/interfaces/catalog.ts del frontend), para que al bajarlo no se pierda
// ninguno (I-12; ver la prueba de contrato en models/contract_test.go).
type ProductCartItem struct {
	// UUID es el del producto en el catálogo de la persona: con él la app junta
	// en un solo renglón el mismo producto agregado dos veces.
	UUID Text `json:"uuid"`
	// Los textos aceptan un número (claves y páginas de productos cargados con la
	// versión 1.0) y lo guardan como texto (ver Text).
	Key     Text    `json:"key"`
	Name    Text    `json:"name"`
	Page    Text    `json:"page"`
	Price   float64 `json:"price"`
	Type    Text    `json:"type"`
	Catalog Text    `json:"catalog"`
	// Sin omitempty: un renglón con cantidad o total 0 regresa con su 0 (sin el
	// campo, la suma de la app da NaN).
	Quantity int     `json:"quantity"`
	Total    float64 `json:"total"`
}

type Catalog struct {
	Catalog     string `json:"catalog"`
	Description string `json:"description"`
	Use         int    `json:"use"`
}
