package main

import (
	"fmt"
	"net/http"
)

func redirectHandler(w http.ResponseWriter, req *http.Request) {
	redirectURL := "http://tunnel.djupvik.dev/" + req.URL.Path

	if len(req.URL.RawQuery) > 0 {
		redirectURL += "?" + req.URL.RawQuery
	}

	http.Redirect(w, req, redirectURL, http.StatusMovedPermanently)
}

func main() {
	http.HandleFunc("/", redirectHandler)

	fmt.Println("Listening on 8080")

	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		panic(err)
	}
}
