package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

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

	ingestFiles, err := os.ReadDir("ingest")
	if err != nil {
		log.Fatal(err)
	}
	ingests := make(map[string][]string)
	for _, dirEntry := range ingestFiles {
		ids := parseIngest("ingest/" + dirEntry.Name())
		name := strings.TrimSuffix(dirEntry.Name(), ".ndjson")
		ingests[name] = ids
	}

	for region, ids := range ingests {
		existingEntries := parseExisting(region)

		var entries []Entry
		for _, id := range ids {
			var entry Entry
			if existing, ok := existingEntries[id]; ok {
				entry = existing
			} else {
				entry = createEntry(id)
			}
			entries = append(entries, entry)
		}

		writeOut(region, entries)
	}
}

func parseIngest(fname string) []string {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var ids []string
	for {
		var id string
		err := dec.Decode(&id)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func parseExisting(region string) map[string]Entry {
	f, err := os.Open("out/" + region + ".ndjson")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var entries map[string]Entry
	for {
		var entry Entry
		err := dec.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		entries[entry.Id] = entry
	}
	return entries
}

func writeOut(region string, entries []Entry) {
	f, err := os.Create("out/" + region + ".ndjson")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			log.Fatal(err)
		}
	}
}

type PictureSize struct {
	Label  string `json:"label"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Source string `json:"source"`
}

type Entry struct {
	Id                  string        `json:"id"`
	Sizes               []PictureSize `json:"sizes"`
	OwnerUsername       string        `json:"ownerUsername"`
	OwnerIcon           string        `json:"ownerIcon"`
	Title               string        `json:"title"`
	Description         string        `json:"description"`
	DateTaken           string        `json:"dateTaken"`
	Latitude            string        `json:"latitude"`
	Longitude           string        `json:"longitude"`
	LocationAccuracy    string        `json:"locationAccuracy"`
	LocationDescription string        `json:"locationDescription"`
	Webpage             string        `json:"url"`
}

func createEntry(id string) Entry {
	var info struct {
		Photo struct {
			Owner struct {
				NSID       string `json:"nsid"`
				Username   string `json:"username"`
				IconServer string `json:"iconserver"`
				IconFarm   int    `json:"iconfarm"`
			} `json:"owner"`
			Title struct {
				Content string `json:"_content"`
			} `json:"title"`
			Description struct {
				Content string `json:"_content"`
			} `json:"description"`
			Dates struct {
				Taken string `json:"taken"`
			} `json:"dates"`
			Location struct {
				Latitude     string `json:"latitude"`
				Longitude    string `json:"longitude"`
				Accuracy     string `json:"accuracy"`
				Neighborhood struct {
					Content string `json:"_content"`
				} `json:"neighborhood"`
				Locality struct {
					Content string `json:"_content"`
				} `json:"locality"`
				County struct {
					Content string `json:"_content"`
				} `json:"county"`
				Region struct {
					Content string `json:"_content"`
				} `json:"region"`
				Country struct {
					Content string `json:"_content"`
				} `json:"country"`
			} `json:"location"`
			URLs struct {
				URL []struct {
					Type    string `json:"type"`
					Content string `json:"_content"`
				} `json:"url"`
			} `json:"urls"`
		} `json:"photo"`
	}
	callFlickr("flickr.photos.getInfo", &info, map[string]string{"photo_id": id})

	var sizes struct {
		Sizes struct {
			Size []PictureSize `json:"size"`
		}
	}
	callFlickr("flickr.photos.getSizes", &sizes, map[string]string{"photo_id": id})

	ownerIcon := "https://www.flickr.com/images/buddyicon.gif"
	if info.Photo.Owner.IconServer != "0" {
		ownerIcon = "https://farm" + fmt.Sprintf("%d", info.Photo.Owner.IconFarm) + ".staticflickr.com/" + info.Photo.Owner.IconServer + "/buddyicons/" + info.Photo.Owner.NSID + ".jpg"
	}

	locationSegments := []string{
		info.Photo.Location.Neighborhood.Content, info.Photo.Location.Locality.Content,
		info.Photo.Location.County.Content, info.Photo.Location.Region.Content, info.Photo.Location.Country.Content}
	locationDescription := strings.Join(locationSegments, ", ")

	webpage := "https://flickr.com/photos/" + info.Photo.Owner.NSID
	if len(info.Photo.URLs.URL) > 0 {
		webpage = info.Photo.URLs.URL[0].Content
	}

	return Entry{
		Id:                  id,
		Sizes:               sizes.Sizes.Size,
		OwnerUsername:       info.Photo.Owner.Username,
		OwnerIcon:           ownerIcon,
		Title:               info.Photo.Title.Content,
		Description:         info.Photo.Description.Content,
		DateTaken:           info.Photo.Dates.Taken,
		Latitude:            info.Photo.Location.Latitude,
		Longitude:           info.Photo.Location.Longitude,
		LocationAccuracy:    info.Photo.Location.Accuracy,
		LocationDescription: locationDescription,
		Webpage:             webpage,
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

	time.Sleep(1 * time.Second)

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
