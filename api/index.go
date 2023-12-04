package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"redirector/db"
)

type URL struct {
	ID   int    `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type URLId struct {
	ID int `json:"id"`
}

var client *db.PrismaClient
var ctx context.Context

func redirectHandler(w http.ResponseWriter, req *http.Request) {
	selectedUrl, err := getSelectedUrl()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirectURL := selectedUrl.URL + req.URL.Path

	if len(req.URL.RawQuery) > 0 {
		redirectURL += "?" + req.URL.RawQuery
	}

	fmt.Println("Proxying to", selectedUrl.URL)

	requestBody, err := ioutil.ReadAll(req.Body)

	if err != nil {
		http.Error(w, "Error reading the request body", http.StatusInternalServerError)
		return
	}

	targetURL, err := url.Parse(redirectURL)

	if err != nil {
		http.Error(w, "Error parsing target URL", http.StatusInternalServerError)
		return
	}

	targetRequest := &http.Request{
		Method: req.Method,
		URL:    targetURL,
		Header: make(http.Header),
		Body:   ioutil.NopCloser(bytes.NewReader(requestBody)),
	}

	for key, values := range req.Header {
		for _, value := range values {
			targetRequest.Header.Add(key, value)
		}
	}

	fmt.Println("Making request", targetRequest)

	client := http.Client{}
	targetResponse, err := client.Do(targetRequest)

	if err != nil {
		http.Error(w, "Error making request to target endpoint", http.StatusInternalServerError)
		return
	}

	defer targetResponse.Body.Close()

	fmt.Println("Response", targetResponse)

	for key, values := range targetResponse.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(targetResponse.StatusCode)

	body, err := ioutil.ReadAll(targetResponse.Body)

	if err != nil {
		http.Error(w, "Error reading target response body", http.StatusInternalServerError)
		return
	}

	fmt.Println("Response body", body)

	w.Write(body)
}

func urlsHandler(w http.ResponseWriter, r *http.Request) {
	urls, err := client.URL.FindMany().With(
		db.URL.Selected.Fetch(),
	).Exec(ctx)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	formattedUrls := make([]map[string]interface{}, len(urls))

	for i, url := range urls {
		hasSelected := len(url.Selected()) > 0
		formattedUrls[i] = map[string]interface{}{
			"id":       url.ID,
			"url":      url.URL,
			"name":     url.Name,
			"selected": hasSelected,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formattedUrls)
}

func isValidURL(s string) bool {
	pattern := `^(http|https)://[a-zA-Z0-9\-.]+(\.[a-zA-Z0-9\-]+)*(:[0-9]+)?(/.*)?$`
	match, err := regexp.MatchString(pattern, s)
	if err != nil {
		return false
	}
	return match
}

func addUrlHandler(w http.ResponseWriter, r *http.Request) {
	var url URL
	err := json.NewDecoder(r.Body).Decode(&url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !isValidURL(url.URL) {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	if url.Name == "" {
		http.Error(w, "URL Name cannot be empty", http.StatusBadRequest)
		return
	}

	createdUrl, err := client.URL.CreateOne(
		db.URL.URL.Set(url.URL),
		db.URL.Name.Set(url.Name),
	).Exec(ctx)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, _ := json.MarshalIndent(createdUrl, "", "  ")
	fmt.Printf("created url: %s\n", result)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(url)
}

func deleteUrlHandler(w http.ResponseWriter, r *http.Request) {
	var urlId URLId
	err := json.NewDecoder(r.Body).Decode(&urlId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	url, err := client.URL.FindUnique(
		db.URL.ID.Equals(urlId.ID),
	).Delete().Exec(ctx)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(url)
}

func selectUrlHandler(w http.ResponseWriter, r *http.Request) {
	var urlId URLId
	err := json.NewDecoder(r.Body).Decode(&urlId)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	selected, err := client.Selected.FindUnique(
		db.Selected.ID.Equals(0),
	).Update(
		db.Selected.SelectedURLID.Set(urlId.ID),
	).Exec(ctx)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(selected)
}

func getSelectedUrl() (*URL, error) {
	selected, err := client.Selected.FindUnique(
		db.Selected.ID.Equals(0),
	).With(
		db.Selected.SelectedURL.Fetch(),
	).Exec(ctx)

	if err != nil {
		return nil, err
	}

	url, _ := selected.SelectedURL()

	if url == nil {
		return nil, nil
	}

	return &URL{
		ID:   url.ID,
		URL:  url.URL,
		Name: url.Name,
	}, nil
}

func selectedUrlHandler(w http.ResponseWriter, r *http.Request) {
	url, err := getSelectedUrl()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if url == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(url)
}

func main() {
	client = db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		panic(err)
	}

	ctx = context.Background()

	if err := client.Prisma.Connect(); err != nil {
		panic(err)
	}

	_, err := client.Selected.UpsertOne(
		db.Selected.ID.Equals(0),
	).Create(
		db.Selected.SelectedURLID.SetOptional(nil),
	).Update().Exec(ctx)

	if err != nil {
		panic(err)
	}

	http.HandleFunc("/urls", urlsHandler)
	http.HandleFunc("/add-url", addUrlHandler)
	http.HandleFunc("/delete-url", deleteUrlHandler)
	http.HandleFunc("/select-url", selectUrlHandler)
	http.HandleFunc("/selected-url", selectedUrlHandler)
	http.HandleFunc("/", redirectHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	fmt.Println("🚀 Listening on :" + port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}
