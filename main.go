package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const (
	tree = "Sequoia"
)

type resp struct {
	MyFavouriteTree string `json:"myFavouriteTree"`
}

func main() {
	staticResponse := resp{tree}

	http.HandleFunc(
		"/tree",
		func(w http.ResponseWriter, r *http.Request) {
			log.Printf("\"%s\" request with header \"%s\" to \"%s\"", r.Method, r.Header, r.URL)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(staticResponse)
		},
	)

	// The /healthz endpoint is added so that kubernetes can evalueate if the pod
	// needs restarting
	http.HandleFunc(
		"/healthz",
		func(w http.ResponseWriter, r *http.Request) {
			log.Println("healthz ping")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		},
	)

	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         ":8090",
	}
	log.Fatal(srv.ListenAndServe())
}
