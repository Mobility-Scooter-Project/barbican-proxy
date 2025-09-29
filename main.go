package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc(("/health"), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc(("/api/v1/secret/{name}"), GetSecretHandler)

	println("Barbican ES proxy listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
