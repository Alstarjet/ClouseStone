package catalog

import (
	"context"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"log"
	"sync"
	"time"
)

// Loader provee los documentos crudos del catálogo (lo implementa
// database.MongoClient; en pruebas, un fake en memoria).
type Loader interface {
	LoadCatalogDocs(ctx context.Context) (models.CatalogDocs, error)
}

const (
	// DefaultTTL es el tiempo que un snapshot se considera vigente.
	DefaultTTL = 5 * time.Minute
	// retryBackoff evita reintentar la carga en cada petición cuando Mongo falla.
	retryBackoff = 15 * time.Second
	loadTimeout  = 20 * time.Second
)

// Service mantiene el snapshot del catálogo en memoria y lo renueva al vencer
// el TTL. Es seguro para uso concurrente.
type Service struct {
	loader Loader
	ttl    time.Duration
	now    func() time.Time

	mu   sync.RWMutex
	snap *Snapshot

	loadMu  sync.Mutex // serializa las recargas
	nextTry time.Time  // no reintentar la carga antes de este momento
}

// NewService crea el servicio. Un ttl <= 0 usa DefaultTTL.
func NewService(loader Loader, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Service{loader: loader, ttl: ttl, now: time.Now}
}

func (s *Service) current() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

func (s *Service) fresh(snap *Snapshot) bool {
	return snap != nil && s.now().Sub(snap.LoadedAt) < s.ttl
}

// Snapshot devuelve el catálogo vigente, recargándolo de Mongo si venció. Si la
// recarga falla pero existe una copia anterior, la sigue sirviendo (y reintenta
// tras un breve intervalo): el catálogo es de solo lectura y cambia poco.
func (s *Service) Snapshot(ctx context.Context) (*Snapshot, error) {
	if snap := s.current(); s.fresh(snap) {
		return snap, nil
	}

	s.loadMu.Lock()
	defer s.loadMu.Unlock()

	// Otra petición pudo haber recargado mientras esperábamos el candado.
	snap := s.current()
	if s.fresh(snap) {
		return snap, nil
	}
	if snap != nil && s.now().Before(s.nextTry) {
		return snap, nil
	}

	loadCtx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()
	docs, err := s.loader.LoadCatalogDocs(loadCtx)
	if err != nil {
		s.nextTry = s.now().Add(retryBackoff)
		if snap != nil {
			log.Printf("catalog: recarga fallida, se sirve la copia anterior: %v", err)
			return snap, nil
		}
		return nil, fmt.Errorf("catalog: cargar catálogo: %w", err)
	}

	fresh := buildSnapshot(docs, s.now())
	s.mu.Lock()
	s.snap = fresh
	s.mu.Unlock()
	return fresh, nil
}
