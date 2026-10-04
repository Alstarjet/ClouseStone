package models

import "go.mongodb.org/mongo-driver/bson"

// CatalogDocs agrupa los documentos crudos de las dos colecciones del catálogo
// curado (solo lectura): products_info y product_kits.
//
// Los documentos se mantienen como bson.M a propósito: el esquema es rico y
// evoluciona fuera del backend, y el API debe devolverlo completo sin perder
// campos. La lógica de consulta extrae solo lo que necesita (ver el paquete
// moduls/catalog).
type CatalogDocs struct {
	Products []bson.M
	Kits     []bson.M
}
