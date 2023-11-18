package charges

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func ChargesUpdateStone(db *database.MongoClient, charges []models.Charge, user models.User) error {
	oldCharges, err := GetOldCharges(db, user)
	if err != nil {
		return err
	}
	newCharges := CompareChargesExist(charges, oldCharges)
	err = SaveNewCharges(db, newCharges, user)
	if err != nil {
		return err
	}
	return nil
}

func GetOldCharges(db *database.MongoClient, user models.User) ([]models.Charge, error) {
	var OldCharges []models.Charge
	//Fechas actual
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	//Fecha previa
	lastMonth := timeNow.AddDate(0, -1, 0)
	yearAfter := lastMonth.Year()
	monthAfter := int(lastMonth.Month())
	docCurrent, err := db.FindMonthCharges(user.Email, year, month)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return OldCharges, err
		}
	}
	docAfter, err := db.FindMonthCharges(user.Email, yearAfter, monthAfter)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return OldCharges, err
		}
	}
	OldCharges = append(OldCharges, docCurrent.Charges...)
	OldCharges = append(OldCharges, docAfter.Charges...)

	return OldCharges, nil
}

func CompareChargesExist(NewCharges []models.Charge, OldCharges []models.Charge) []models.Charge {
	var realNewCharges []models.Charge
	for _, paymentNew := range NewCharges {
		var exits = false
		for _, paymentOld := range OldCharges {
			if paymentNew.Uuid == paymentOld.Uuid {
				exits = true
			}
		}
		if !exits {
			realNewCharges = append(realNewCharges, paymentNew)
		}
	}
	return realNewCharges
}

func SaveNewCharges(db *database.MongoClient, NewCharges []models.Charge, user models.User) error {
	//Fechas actual
	timeNow := time.Now()
	year := timeNow.Year()
	month := int(timeNow.Month())
	docCurrent, err := db.FindMonthCharges(user.Email, year, month)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			docCurrent.ID = primitive.NewObjectID()
			docCurrent.Month = month
			docCurrent.Year = year
			docCurrent.UserEmail = user.Email
			docCurrent.Charges = NewCharges
			_, err = db.AddMonthCharges(&docCurrent)
			if err != nil {
				fmt.Println("Error addMonthCharges", err)
				return err
			}
			return nil
		} else {
			return err
		}
	} else {
		docCurrent.Charges = append(docCurrent.Charges, NewCharges...)
		_, err = db.UpdateMonthCharges(&docCurrent)
		if err != nil {
			fmt.Println("Error al actualizar reporte", err)
			return err
		}
		return nil
	}
}
