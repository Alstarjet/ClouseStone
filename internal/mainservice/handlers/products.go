package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"io"
	"log"
	"net/http"
)

func Products(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.Product
		req, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(req, &newData)
		produc, err := db.FindProduct(newData.Key)
		if err == nil && produc.Key != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Email ya en Uso, prueba con otro"))
			return
		}
		reques, err := db.AddProduct(&newData)
		if err != nil {
			log.Println(err)
		}
		respJSON, err := json.Marshal(reques)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
	})
}
func FindAllProducts(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.Product
		req, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(req, &newData)
		produc, err := db.FindProduct(newData.Key)
		if err == nil && produc.Key != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("Email ya en Uso, prueba con otro"))
			return
		}
		reques, err := db.FindAllProducts()
		if err != nil {
			log.Println(err)
		}
		respJSON, err := json.Marshal(reques)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
	})
}
