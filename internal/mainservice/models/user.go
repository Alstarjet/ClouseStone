package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID         primitive.ObjectID `bson:"_id"`
	Name       string             `bson:"name" json:"name"`
	LastName   string             `bson:"lastname" json:"lastname"`
	Email      string             `bson:"email" json:"email"`
	Age        int64              `bson:"age" json:"age"`
	Password   string             `bson:"password" json:"password"`
	Phone      int64              `bson:"phone" json:"phone"`
	TypeClient string             `bson:"typeclient"`
	// Constancia de aceptación de los Términos, el Aviso de privacidad y la
	// Política de cookies al registrarse: fecha del servidor y versión aceptada.
	// json:"-" impide que el cliente las escriba directamente (ver handlers/terms.go).
	TermsAcceptedAt *time.Time `bson:"termsacceptedat,omitempty" json:"-"`
	TermsVersion    string     `bson:"termsversion,omitempty" json:"-"`
	// Vigencia del plan de pago: cuándo vence. Se pone a mano junto con el plan
	// (typeclient) al recibir el pago; vacía = sin vencimiento. Al vencer, el plan
	// también se cambia a mano. json:"-" impide que el registro la escriba.
	PlanExpiresAt *time.Time `bson:"planexpiresat,omitempty" json:"-"`
	// Tope de clientes propio de la cuenta, para ampliarle el plan de pago sin
	// cambiar el código (ver AccountLimit). Se pone a mano en la base; json:"-"
	// impide que el registro lo escriba.
	ClientLimit AccountLimit `bson:"clientlimit,omitempty" json:"-"`
}
type UserDevices struct {
	ID          primitive.ObjectID `bson:"_id"`
	UserMongoID string             `bson:"usermongoid"`
	UserName    string             `bson:"username"`
	UserEmail   string             `bson:"useremail"`
	Devices     []Device           `bson:"devices"`
}
type Device struct {
	UUID         string       `bson:"uuid"`
	ChargeIDs    []string     `bson:"chargeids"`
	PaymentIDs   []string     `bson:"paymentids"`
	ClientIDs    []string     `bson:"clientids"`
	OrderIDs     []string     `bson:"orderids"`
	Refreshtoken Refreshtoken `bson:"refreshtoken"`
}
type Refreshtoken struct {
	Token   string    `bson:"token"`
	DateEnd time.Time `bson:"dateend"`
}
