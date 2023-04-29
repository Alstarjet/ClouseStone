package handlers

import "net/http"

func Hello() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!2"))
	})
}