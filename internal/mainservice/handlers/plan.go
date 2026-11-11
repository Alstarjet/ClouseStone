package handlers

import (
	"financial-Assistant/internal/mainservice/models"
	"financial-Assistant/internal/mainservice/moduls/plans"
	"net/http"
)

// withPlan agrega a la respuesta del login y de /GetJwt el plan del usuario, sus
// reglas (moduls/plans) y su vigencia.
func withPlan(response models.JWTresponce, user models.User) models.JWTresponce {
	rules := plans.For(user.TypeClient)
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
