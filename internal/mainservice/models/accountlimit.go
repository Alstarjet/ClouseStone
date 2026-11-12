package models

import (
	"math"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// AccountLimit es el tope de clientes propio de una cuenta (users.clientlimit). Se
// pone a mano en la base para ampliarle el plan de pago a una cuenta sin cambiar el
// código: con 500, esa cuenta registra hasta 500 clientes en vez de los 300 del plan.
// Sin él, o en la versión gratis, vale el tope del plan (plans.ForAccount).
//
// Como se escribe a mano, se lee con tolerancia y nunca da error, para que un valor
// mal escrito no impida el login. Cuenta un número entero positivo, como lo guarde
// mongosh, Compass o Atlas (int32, int64 o double sin decimales). Cualquier otra cosa
// (texto, cero, negativo, con decimales, verdadero/falso) es un valor no válido: no
// cuenta y withPlan lo anota en el log con la cuenta. null o sin campo: no tiene.
type AccountLimit int

// invalidAccountLimit marca un valor que está en la base pero no sirve.
const invalidAccountLimit AccountLimit = -1

// UnmarshalBSONValue lee users.clientlimit (bson.ValueUnmarshaler).
func (l *AccountLimit) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	raw := bson.RawValue{Type: t, Value: data}
	var n float64
	ok := false
	switch t {
	case bsontype.Null, bsontype.Undefined:
		*l = 0
		return nil
	case bsontype.Int32:
		var v int32
		v, ok = raw.Int32OK()
		n = float64(v)
	case bsontype.Int64:
		var v int64
		v, ok = raw.Int64OK()
		n = float64(v)
	case bsontype.Double:
		n, ok = raw.DoubleOK()
	}
	if !ok || n < 1 || n > math.MaxInt32 || n != math.Trunc(n) {
		*l = invalidAccountLimit
		return nil
	}
	*l = AccountLimit(n)
	return nil
}

// Value es el tope propio de la cuenta, o 0 si no tiene (o si el de la base no sirve).
func (l AccountLimit) Value() int {
	if l > 0 {
		return int(l)
	}
	return 0
}

// Invalid dice si users.clientlimit está en la base pero no sirve.
func (l AccountLimit) Invalid() bool {
	return l == invalidAccountLimit
}
