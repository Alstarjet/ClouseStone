package handlers

import (
	"financial-Assistant/internal/mainservice/models"
	"unicode/utf8"
)

// Topes de tamaño de lo que manda alguien sin sesión (/register y /login). Sin
// ellos, cada registro podía guardar un documento de hasta 1 MB, y unas 500
// peticiones llenaban la base (evaluación de seguridad del 12-nov-2026, S-02).
const (
	// maxNameLength: caracteres del nombre y del apellido.
	maxNameLength = 80
	// maxEmailLength: el máximo de una dirección de correo (RFC 5321).
	maxEmailLength = 254
	// maxPasswordBytes: bcrypt sólo usa los primeros 72 bytes, y con más da error.
	maxPasswordBytes = 72
	// maxDeviceLength: la app manda un UUID de 36 caracteres (S-05).
	maxDeviceLength = 64
)

// registerLimits revisa los topes de tamaño del registro. Regresa el mensaje del
// error, o vacío si todo cabe.
func registerLimits(user models.User) string {
	switch {
	case utf8.RuneCountInString(user.Name) > maxNameLength:
		return "el nombre es demasiado largo (máximo 80 caracteres)"
	case utf8.RuneCountInString(user.LastName) > maxNameLength:
		return "el apellido es demasiado largo (máximo 80 caracteres)"
	case len(user.Email) > maxEmailLength:
		return "el email es demasiado largo"
	case len(user.Password) > maxPasswordBytes:
		return "la contraseña es demasiado larga (máximo 72 caracteres)"
	}
	return ""
}
