package handlers

import (
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"log"
	"net/http"
)

// withPlan agrega a la respuesta del login y de /GetJwt el plan del usuario, sus
// reglas (moduls/plans) con el tope de clientes propio de la cuenta si tiene uno
// (users.clientlimit), y su vigencia. Un clientlimit mal escrito no cuenta y se
// anota en el log con la cuenta, para corregirlo en la base.
func withPlan(response models.JWTresponce, user models.User) models.JWTresponce {
	if user.ClientLimit.Invalid() {
		log.Printf("plans: users.clientlimit no válido en la cuenta %s; se usa el tope del plan", user.ID.Hex())
	}
	rules := plans.ForAccount(user.TypeClient, user.ClientLimit.Value())
	response.TypeClient = user.TypeClient
	response.CloudBackup = rules.CloudBackup
	response.ClientLimit = rules.ClientLimit
	response.PlanExpiresAt = user.PlanExpiresAt
	return response
}

// allowsCloudBackup dice si el plan del usuario respalda en línea. Si no (la versión
// gratis o un plan desconocido), responde 403 plan_without_backup: esa cuenta no
// guarda nada en la nube. No es el tope de clientes, que lo cuida el frontend.
func allowsCloudBackup(w http.ResponseWriter, user models.User) bool {
	if plans.For(user.TypeClient).CloudBackup {
		return true
	}
	http.Error(w, `{"error":"plan_without_backup"}`, http.StatusForbidden)
	return false
}
