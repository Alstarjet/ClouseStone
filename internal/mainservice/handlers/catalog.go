package handlers

import (
	"encoding/json"
	"errors"
	"financial-Assistant/internal/mainservice/moduls/catalog"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// Catálogo curado de productos y kits (solo lectura). Los endpoints cuelgan de
// /catalog y exigen access token (AuthMiddleware en route.go): cualquier usuario
// autenticado puede consultarlos.

const (
	defaultListLimit = 50
	maxListLimit     = 200
	catalogCacheCtl  = "private, max-age=60"
)

// writeJSON escribe una respuesta JSON con el código dado.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("catalog: encode response: %v", err)
	}
}

// writeJSONError escribe un error con la forma {"error":"..."} y Content-Type JSON.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// catalogFail traduce un error del módulo catalog al código HTTP adecuado.
func catalogFail(w http.ResponseWriter, where string, err error) {
	var perr *catalog.ParamError
	switch {
	case errors.As(err, &perr):
		writeJSONError(w, http.StatusBadRequest, perr.Error())
	case errors.Is(err, catalog.ErrNotFound):
		writeJSONError(w, http.StatusNotFound, "not found")
	default:
		log.Printf("%s: %v", where, err)
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
	}
}

// queryList lee un parámetro que admite valores separados por comas y/o
// repetidos: ?need=a,b&need=c → [a b c].
func queryList(r *http.Request, name string) []string {
	var out []string
	for _, raw := range r.URL.Query()[name] {
		for _, v := range strings.Split(raw, ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// intParam lee un entero acotado a [min, max]; vacío = def; no numérico = error.
func intParam(r *http.Request, name string, def, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: must be an integer", name, raw)
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v, nil
}

// boolParam lee un booleano ("false", "0", "no" = falso); vacío = def.
func boolParam(r *http.Request, name string, def bool) bool {
	switch strings.ToLower(r.URL.Query().Get(name)) {
	case "":
		return def
	case "false", "0", "no":
		return false
	default:
		return true
	}
}

// listParams lee limit y skip de la query.
func listParams(r *http.Request) (limit, skip int, err error) {
	if limit, err = intParam(r, "limit", defaultListLimit, 1, maxListLimit); err != nil {
		return 0, 0, err
	}
	if skip, err = intParam(r, "skip", 0, 0, 1<<30); err != nil {
		return 0, 0, err
	}
	return limit, skip, nil
}

// withSnapshot carga el catálogo y ejecuta el handler; centraliza el manejo de
// errores de carga y las cabeceras comunes.
func withSnapshot(svc *catalog.Service, where string, fn func(w http.ResponseWriter, r *http.Request, snap *catalog.Snapshot)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap, err := svc.Snapshot(r.Context())
		if err != nil {
			catalogFail(w, where, err)
			return
		}
		w.Header().Set("Cache-Control", catalogCacheCtl)
		fn(w, r, snap)
	})
}

func writeList(w http.ResponseWriter, items any, total, limit, skip int) {
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "skip": skip})
}

// CatalogProducts: GET /catalog/products — lista de productos con filtros
// (need, rubro, type, application, min_relevance, limit, skip).
func CatalogProducts(svc *catalog.Service) http.Handler {
	return withSnapshot(svc, "CatalogProducts", func(w http.ResponseWriter, r *http.Request, snap *catalog.Snapshot) {
		limit, skip, err := listParams(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		minRel := 0.0
		if raw := r.URL.Query().Get("min_relevance"); raw != "" {
			if minRel, err = strconv.ParseFloat(raw, 64); err != nil || minRel < 0 || minRel > 1 {
				writeJSONError(w, http.StatusBadRequest, `invalid min_relevance: must be a number between 0 and 1`)
				return
			}
		}
		q := r.URL.Query()
		items, total, err := snap.ProductList(catalog.ProductFilter{
			Needs: queryList(r, "need"),
			Rubro: q.Get("rubro"), Type: q.Get("type"), Application: q.Get("application"),
			MinRelevance: minRel, Limit: limit, Skip: skip,
		})
		if err != nil {
			catalogFail(w, "CatalogProducts", err)
			return
		}
		writeList(w, items, total, limit, skip)
	})
}

// CatalogProduct: GET /catalog/products/{id} — un producto completo, por clave
// del catálogo (cualquier variante) o por slug.
func CatalogProduct(svc *catalog.Service) http.Handler {
	return withSnapshot(svc, "CatalogProduct", func(w http.ResponseWriter, r *http.Request, snap *catalog.Snapshot) {
		doc, err := snap.Product(mux.Vars(r)["id"])
		if err != nil {
			catalogFail(w, "CatalogProduct", err)
			return
		}
		writeJSON(w, http.StatusOK, doc)
	})
}

// CatalogKits: GET /catalog/kits — kits curados (need, perfil, expand, limit, skip).
func CatalogKits(svc *catalog.Service) http.Handler {
	return withSnapshot(svc, "CatalogKits", func(w http.ResponseWriter, r *http.Request, snap *catalog.Snapshot) {
		limit, skip, err := listParams(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		items, total, err := snap.KitList(catalog.KitFilter{
			Needs: queryList(r, "need"), Perfiles: queryList(r, "perfil"),
			Expand: boolParam(r, "expand", true), Limit: limit, Skip: skip,
		})
		if err != nil {
			catalogFail(w, "CatalogKits", err)
			return
		}
		writeList(w, items, total, limit, skip)
	})
}

// CatalogKit: GET /catalog/kits/{slug} — un kit con sus productos resueltos.
func CatalogKit(svc *catalog.Service) http.Handler {
	return withSnapshot(svc, "CatalogKit", func(w http.ResponseWriter, r *http.Request, snap *catalog.Snapshot) {
		doc, err := snap.Kit(mux.Vars(r)["slug"])
		if err != nil {
			catalogFail(w, "CatalogKit", err)
			return
		}
		writeJSON(w, http.StatusOK, doc)
	})
}
