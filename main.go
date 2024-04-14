package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

var flickrAPIKey string

func init() {
	err := godotenv.Load(".local.env")
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}

	flickrAPIKey = os.Getenv("FLICKR_API_KEY")
	if flickrAPIKey == "" {
		log.Fatal("FLICKR_API_KEY not set")
	}

}

func main() {
	outDir := "out"
	if err := os.MkdirAll(outDir, 0750); err != nil {
		log.Fatal(err)
	}
}

func callFlickr(method string, resp any, params map[string]string) {
	params["method"] = method
	params["api_key"] = flickrAPIKey
	params["format"] = "json"
	params["nojsoncallback"] = "1"

	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}

	r := url.URL{
		Scheme:   "https",
		Host:     "www.flickr.com",
		Path:     "/services/rest",
		RawQuery: query.Encode(),
	}

	log.Printf("Calling Flickr API: %s", r.String())

	httpResp, err := http.Get(r.String())
	if err != nil {
		log.Fatal(err)
	}
	if httpResp.StatusCode != http.StatusOK {
		log.Fatalf("HTTP status %d", httpResp.StatusCode)
	}

	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		log.Fatal(err)
		return
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		log.Fatal(err)
		return
	}
}
