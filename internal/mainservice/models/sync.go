package models

import "time"

// SyncChanges agrupa los documentos de cada colección que viajan en una
// sincronización, tanto al subir (cliente → servidor) como al bajar
// (servidor → cliente). Las órdenes reutilizan el modelo Charge.
type SyncChanges struct {
	Clients  []Client  `json:"clients"`
	Charges  []Charge  `json:"charges"`
	Orders   []Charge  `json:"orders"`
	Payments []Payment `json:"payments"`
	Products []Product `json:"products"`
}

// SyncRequest es el cuerpo de POST /sync.
//
// Since es el cursor de la última sincronización (el ServerTime devuelto la vez
// anterior). Es nil en la primera sincronización del dispositivo, en cuyo caso
// el servidor devuelve el snapshot completo de registros activos.
//
// Changes contiene los cambios locales que el dispositivo quiere subir. Para
// eliminar un registro se envía con Status = "deleted" (soft delete).
type SyncRequest struct {
	Since   *time.Time  `json:"since"`
	Changes SyncChanges `json:"changes"`
}

// SyncResponse es la respuesta de POST /sync.
//
// ServerTime es el nuevo cursor: el cliente lo guarda y lo envía como Since en
// la siguiente sincronización. Siempre proviene del reloj del servidor.
//
// Changes contiene todo lo que cambió en el servidor desde Since (incluye
// tombstones con Status = "deleted").
type SyncResponse struct {
	ServerTime time.Time   `json:"servertime"`
	Changes    SyncChanges `json:"changes"`
}
