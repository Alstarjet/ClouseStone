package handlers

import (
	"encoding/json"
	"financial-Assistant/internal/mainservice/database"
	"financial-Assistant/internal/mainservice/models"
	"io/ioutil"
	"log"
	"net/http"
)

func Hello() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!2"))
	})
}
func Register(db *database.MongoClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var newData models.Request
		req, _ := ioutil.ReadAll(r.Body)
		_ = json.Unmarshal(req, &newData)
		err := db.InsertUser(&newData)
		if err != nil {
			log.Println(err)
		}
		w.Write([]byte("Hello, World!2"))
	})
}
