package handlers

import (
	"context"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Tope de clientes en el respaldo. La app ya no deja registrar más clientes que el
// tope de la cuenta (el del plan o el propio, plans.ForAccount), pero dos
// dispositivos sin internet pueden registrar cada uno los que le quedan y, al
// respaldar, pasarse juntos. Aquí se revisa sólo cuando el lote trae clientes:
//
//   - Un cliente que ya está en la nube siempre se actualiza, aunque la cuenta vaya
//     pasada del tope. Sólo cuentan los clientes nuevos y activos.
//   - Los nuevos entran en el orden en que se registraron (createat) hasta llenar
//     los lugares libres. Los que sobran se rechazan con el código client_limit y se
//     quedan pendientes en el dispositivo; el resto del lote se guarda normal. Los
//     cargos, pedidos y pagos nunca se rechazan por esto.
//   - Cuenta igual que la app: clientes con status "active" (la lista negra cuenta,
//     los borrados no).
//
// No es atómico a propósito: dos respaldos exactamente a la vez podrían pasarse por
// uno o dos clientes. Es raro y se ve en la consola (pantalla Uso).

// clientLimitStore es lo que necesita la revisión del tope.
type clientLimitStore interface {
	ExistingUUIDs(ctx context.Context, collection, usermongoid string, uuids []string) (map[string]bool, error)
	CountActiveClients(ctx context.Context, usermongoid string) (int64, error)
}

// clientLimitCode es el código de un cliente que no cupo (models.RejectedRecord.Code).
const clientLimitCode = "client_limit"

// applyClientLimit regresa los clientes del lote que sí se guardan y los rechazos de
// los nuevos que ya no caben. Sin clientes nuevos y activos no cuenta nada.
func applyClientLimit(ctx context.Context, store clientLimitStore, user models.User, ups []database.Upsert) ([]database.Upsert, []models.RejectedRecord, error) {
	if len(ups) == 0 {
		return ups, nil, nil
	}
	usermongoid := user.ID.Hex()
	uuids := make([]string, 0, len(ups))
	listed := map[string]bool{}
	for _, up := range ups {
		if !listed[up.UUID] {
			listed[up.UUID] = true
			uuids = append(uuids, up.UUID)
		}
	}
	existing, err := store.ExistingUUIDs(ctx, "clients", usermongoid, uuids)
	if err != nil {
		return nil, nil, err
	}

	type newClient struct {
		uuid     string
		createAt time.Time
	}
	var news []newClient
	counted := map[string]bool{}
	for _, up := range ups {
		if existing[up.UUID] || counted[up.UUID] || !activeClient(up) {
			continue
		}
		counted[up.UUID] = true
		news = append(news, newClient{up.UUID, createdAt(up)})
	}
	if len(news) == 0 {
		return ups, nil, nil
	}

	active, err := store.CountActiveClients(ctx, usermongoid)
	if err != nil {
		return nil, nil, err
	}
	limit := plans.ForAccount(user.TypeClient, user.ClientLimit.Value()).ClientLimit
	room := limit - int(active)
	if room < 0 {
		room = 0
	}
	if len(news) <= room {
		return ups, nil, nil
	}

	sort.SliceStable(news, func(a, b int) bool { return news[a].createAt.Before(news[b].createAt) })
	over := map[string]bool{}
	rejected := make([]models.RejectedRecord, 0, len(news)-room)
	for _, client := range news[room:] {
		over[client.uuid] = true
		rejected = append(rejected, models.RejectedRecord{
			Store:  "clients",
			UUID:   client.uuid,
			Reason: fmt.Sprintf("pasa el tope de %d clientes de la cuenta", limit),
			Code:   clientLimitCode,
		})
	}
	kept := make([]database.Upsert, 0, len(ups))
	for _, up := range ups {
		if !over[up.UUID] {
			kept = append(kept, up)
		}
	}
	return kept, rejected, nil
}

// upsertField es el valor de un campo como quedaría en un documento nuevo (Set o,
// si no está ahí, SetOnInsert).
func upsertField(up database.Upsert, field string) any {
	if value, ok := up.Set[field]; ok {
		return value
	}
	return up.SetOnInsert[field]
}

// activeClient dice si el cliente cuenta para el tope (status "active").
func activeClient(up database.Upsert) bool {
	status, _ := upsertField(up, "status").(string)
	return status == "active"
}

// createdAt es cuándo se registró el cliente en el dispositivo (sin fecha: el
// principio de los tiempos, así que entra primero).
func createdAt(up database.Upsert) time.Time {
	switch value := upsertField(up, "createat").(type) {
	case primitive.DateTime:
		return value.Time()
	case time.Time:
		return value
	}
	return time.Time{}
}
