package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK from mock backend, path=%s\n", r.URL.Path)
	})

	log.Println("mock backend listening on :9001")
	log.Fatal(http.ListenAndServe(":9001", nil))
}
