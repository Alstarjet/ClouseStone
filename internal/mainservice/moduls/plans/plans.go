// Package plans dice qué permite cada plan de Gluzie. Es el único lugar con las
// reglas: un plan nuevo (otro nivel de pago) se agrega aquí y nada más.
//
// El plan de cada usuario es su typeclient (models.User.TypeClient). Se asigna
// "Quartz" al registrarse y se cambia a mano en la base cuando paga. Para darle más
// clientes a una cuenta de pago no hace falta un plan nuevo: se le pone su propio
// tope en la base (users.clientlimit, ver ForAccount).
package plans

import "log"

// Rules son las reglas de un plan.
type Rules struct {
	// CloudBackup: respaldo en línea, sincronización y varios dispositivos. Sin él,
	// /UploadData responde 403 plan_without_backup y no guarda nada.
	CloudBackup bool
	// ClientLimit: tope de clientes (el del plan, o el propio de la cuenta con
	// ForAccount). El backend sólo lo informa en el login y en /GetJwt; lo cuida el
	// frontend.
	ClientLimit int
}

// Free es el plan de la versión gratis, el que se asigna al registrarse.
const Free = "Quartz"

// Paid es el plan de pago.
const Paid = "Amethyst"

var rules = map[string]Rules{
	Free: {CloudBackup: false, ClientLimit: 50},
	Paid: {CloudBackup: true, ClientLimit: 300},
}

// For regresa las reglas del plan. Un plan desconocido o vacío se trata como la
// versión gratis, para no abrir ningún hueco, y se anota en el log.
func For(typeclient string) Rules {
	if r, ok := rules[typeclient]; ok {
		return r
	}
	log.Printf("plans: plan desconocido %q; se trata como la versión gratis", typeclient)
	return rules[Free]
}

// ForAccount regresa las reglas del plan con el tope de clientes propio de la cuenta
// (users.clientlimit), si tiene uno: así se le amplía el plan a una cuenta desde la
// base, sin cambiar el código. Sólo cuenta en un plan de pago (con respaldo en
// línea): si la cuenta pasa a la versión gratis, vuelve sola al tope de la versión
// gratis. accountLimit 0 o menos: no tiene.
func ForAccount(typeclient string, accountLimit int) Rules {
	r := For(typeclient)
	if r.CloudBackup && accountLimit > 0 {
		r.ClientLimit = accountLimit
	}
	return r
}
