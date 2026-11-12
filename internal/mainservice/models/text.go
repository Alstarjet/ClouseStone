package models

import (
	"encoding/json"
	"reflect"
)

// Text es un campo de texto que también acepta un número y lo guarda como el
// texto que llegó (12 → "12", 12.50 → "12.50"). Así, una celda de Excel con sólo
// dígitos (colonia, ciudad, día de cobro) o la clave de un producto cargado con
// la versión 1.0 de la app no hace que se rechace el registro (I-02). En Mongo y
// en las respuestas siempre es texto.
//
// Cualquier otro tipo (verdadero/falso, objeto, lista) sigue siendo un error de
// tipo, con el nombre del campo, como en un string normal.
type Text string

// UnmarshalJSON acepta un texto o un número. null no cambia nada, igual que en
// un string normal.
func (t *Text) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*t = Text(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*t = Text(n)
		return nil
	}
	// Con el tipo de error de siempre, el decodificador le agrega el campo
	// ("products.key") y el respaldo puede decir dónde estuvo el problema.
	return &json.UnmarshalTypeError{Value: jsonKind(data), Type: reflect.TypeOf("")}
}

// jsonKind nombra el tipo de un valor JSON que no es texto ni número.
func jsonKind(data []byte) string {
	if len(data) > 0 {
		switch data[0] {
		case 't', 'f':
			return "bool"
		case '{':
			return "object"
		case '[':
			return "array"
		}
	}
	return "value"
}
