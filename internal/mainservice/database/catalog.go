package database

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Colecciones del catálogo curado de productos (solo lectura). Son un servicio
// independiente: no participan en la sincronización ni dependen de usuarios.
const (
	collProductsInfo = "products_info"
	collProductKits  = "product_kits"
)

// LoadCatalogDocs lee completas las dos colecciones del catálogo. Son pequeñas
// (cientos de documentos), por eso el servicio las carga enteras a un caché en
// memoria en lugar de consultar Mongo en cada petición. Una colección que aún
// no existe devuelve una lista vacía.
func (mc *MongoClient) LoadCatalogDocs(ctx context.Context) (models.CatalogDocs, error) {
	var docs models.CatalogDocs
	targets := []struct {
		name string
		dst  *[]bson.M
	}{
		{collProductsInfo, &docs.Products},
		{collProductKits, &docs.Kits},
	}

	for _, t := range targets {
		cursor, err := mc.client.Database(DataBase).Collection(t.name).Find(ctx, bson.M{})
		if err != nil {
			return models.CatalogDocs{}, fmt.Errorf("LoadCatalogDocs: leer %q: %w", t.name, err)
		}
		if err := cursor.All(ctx, t.dst); err != nil {
			return models.CatalogDocs{}, fmt.Errorf("LoadCatalogDocs: decodificar %q: %w", t.name, err)
		}
	}
	return docs, nil
}

// EnsureCatalogIndexes crea los índices del catálogo (idempotente: no hace nada
// si ya existen). Se invoca al arrancar el servicio.
//
//   - Identidad única: products_info.slug y products_info.variants.key (cada clave
//     del catálogo pertenece a un solo producto) y product_kits.slug.
//   - Consulta: products_info.needs.need y .rubros; product_kits.needs, .perfiles
//     e items.key.
//
// Es best-effort: un índice que falle (p. ej. por duplicados previos en la
// data) se registra y se continúa, sin bloquear el arranque del servicio.
func (mc *MongoClient) EnsureCatalogIndexes(ctx context.Context) error {
	unique := func(name string) *options.IndexOptions {
		return options.Index().SetUnique(true).SetName(name)
	}
	byColl := map[string][]mongo.IndexModel{
		collProductsInfo: {
			{Keys: bson.D{{Key: "slug", Value: 1}}, Options: unique("slug_unique")},
			{Keys: bson.D{{Key: "variants.key", Value: 1}}, Options: unique("variants_key_unique")},
			{Keys: bson.D{{Key: "needs.need", Value: 1}}},
			{Keys: bson.D{{Key: "rubros", Value: 1}}},
		},
		collProductKits: {
			{Keys: bson.D{{Key: "slug", Value: 1}}, Options: unique("slug_unique")},
			{Keys: bson.D{{Key: "needs", Value: 1}}},
			{Keys: bson.D{{Key: "perfiles", Value: 1}}},
			{Keys: bson.D{{Key: "items.key", Value: 1}}},
		},
	}

	for name, indexes := range byColl {
		coll := mc.client.Database(DataBase).Collection(name)
		for _, idx := range indexes {
			// Uno por uno: si un índice único falla por datos duplicados, los demás se crean igual.
			if _, err := coll.Indexes().CreateOne(ctx, idx); err != nil {
				log.Printf("EnsureCatalogIndexes: índice %v de %q no creado (revisar duplicados): %v", idx.Keys, name, err)
			}
		}
	}
	return nil
}
