package payments

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func PaymentsUpdateStone(db *database.MongoClient, payments []models.Payment, user models.User) error {
	oldPayments, err := GetOldPayments(db, user)
	if err != nil {
		return err
	}
	newPayments := ComparePaymentsExist(payments, oldPayments)
	err = SaveNewPayments(db, newPayments, user)
	if err != nil {
		return err
	}
	return nil
}

func GetOldPayments(db *database.MongoClient, user models.User) ([]models.Payment, error) {
	var OldPayments []models.Payment
	//Fechas actual
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	//Fecha previa
	lastMonth := timeNow.AddDate(0, -1, 0)
	yearAfter := lastMonth.Year()
	monthAfter := int(lastMonth.Month())
	docCurrent, err := db.FindMonthPayments(user.Email, year, month)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return OldPayments, err
		}
	}
	docAfter, err := db.FindMonthPayments(user.Email, yearAfter, monthAfter)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return OldPayments, err
		}
	}
	OldPayments = append(OldPayments, docCurrent.Payments...)
	OldPayments = append(OldPayments, docAfter.Payments...)

	return OldPayments, nil
}

func ComparePaymentsExist(NewPayments []models.Payment, OldPayments []models.Payment) []models.Payment {
	var realNewPayments []models.Payment
	for _, paymentNew := range NewPayments {
		var exits = false
		for _, paymentOld := range OldPayments {
			if paymentNew.Uuid == paymentOld.Uuid {
				exits = true
			}
		}
		if !exits {
			realNewPayments = append(realNewPayments, paymentNew)
		}
	}
	return realNewPayments
}

func SaveNewPayments(db *database.MongoClient, NewPayments []models.Payment, user models.User) error {
	//Fechas actual
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	docCurrent, err := db.FindMonthPayments(user.Email, year, month)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			docCurrent.ID = primitive.NewObjectID()
			docCurrent.Month = month
			docCurrent.Year = year
			docCurrent.UserEmail = user.Email
			docCurrent.Payments = NewPayments
			_, err = db.AddMonthPayments(&docCurrent)
			if err != nil {
				fmt.Println("Error addMonthPayments", err)
				return err
			}
			return nil
		} else {
			return err
		}
	} else {
		docCurrent.Payments = append(docCurrent.Payments, NewPayments...)
		_, err = db.UpdateMonthPayments(&docCurrent)
		if err != nil {
			fmt.Println("Error al actualizar reporte", err)
			return err
		}
		return nil
	}
}
