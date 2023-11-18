package reports

import (
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func FindOrCreateReport(db *database.MongoClient, userEmail string, clientUuid string, year int, month int, AfterReport models.MonthReport) (models.MonthReport, error) {
	monthReport, err := db.FindReport(userEmail, clientUuid, year, month)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			monthReport.UserEmail = userEmail
			monthReport.ID = primitive.NewObjectID()
			monthReport.Month = month
			monthReport.Year = year
			monthReport.LastDebt = lastDebtCalculator(AfterReport)
			monthReport.Charges = []models.Charge{}
			monthReport.Payments = []models.Payment{}
			_, err := db.AddReport(&monthReport)
			if err != nil {
				return monthReport, err
			}
			return monthReport, nil
		} else {
			return monthReport, err
		}
	}
	return monthReport, err
}

func lastDebtCalculator(AfterReporte models.MonthReport) float64 {
	var paymentSum float64
	var chargeSum float64
	for _, payment := range AfterReporte.Payments {
		paymentSum = paymentSum + payment.Amount
	}
	for _, charge := range AfterReporte.Charges {
		chargeSum = chargeSum + charge.FinalPrice
	}
	balance := AfterReporte.LastDebt - paymentSum + chargeSum
	return balance
}
