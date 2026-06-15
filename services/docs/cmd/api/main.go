package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/s4f4y4t/go-microservice/services/docs"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	sub, err := fs.Sub(docs.FS, "static")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(sub)))

	log.Printf("docs server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
