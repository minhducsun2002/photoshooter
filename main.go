package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Name    string
	Content []byte
}

type UploadResponse struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
	URL  string `json:"url"`
}

var queue = make(chan Entry, 300)
var client = http.Client{}

// backoff counter
var lastDuration = 0 * time.Second
var maxDuration = 15 * time.Minute

func push(apiKey string, endpoint string, uuid string) {
	for {
		l := len(queue)
		if l == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		// wait
		lastDuration = min(lastDuration, maxDuration)
		time.Sleep(lastDuration)

		entry := <-queue

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		mimeType := mime.TypeByExtension(filepath.Ext(entry.Name))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		headers := make(textproto.MIMEHeader)
		headers.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", entry.Name))
		headers.Set("Content-Type", mimeType)
		content, _ := writer.CreatePart(headers)
		_, _ = content.Write(entry.Content)

		_ = writer.Close()

		req, _ := http.NewRequest("POST", endpoint, body)
		req.Header.Add("X-API-Key", apiKey)
		req.Header.Add("albumuuid", uuid)
		req.Header.Add("Content-Type", writer.FormDataContentType())

		response, err := client.Do(req)
		if err != nil {
			log.Printf("Error uploading file %s : %s", entry.Name, err)
			queue <- entry
			if lastDuration == 0*time.Second {
				lastDuration = 1 * time.Second
			} else {
				lastDuration *= 2
			}
			continue
		}

		if response.StatusCode != 200 {
			log.Printf("Error uploading file %s : Response status was %d", entry.Name, response.StatusCode)
			queue <- entry
			if lastDuration == 0*time.Second {
				lastDuration = 1 * time.Second
			} else {
				lastDuration *= 3
			}
			continue
		}

		r := new(UploadResponse)
		err = json.NewDecoder(response.Body).Decode(r)
		if err != nil {
			log.Printf("Error unmarshalling response : %s", err)
			continue
		}

		log.Printf("Uploaded as %s", r.Name)
		lastDuration = 0 * time.Second
	}
}

func upload(w http.ResponseWriter, req *http.Request) {
	name := req.Header.Get("Name")

	if name == "" {
		w.WriteHeader(400)
		return
	}

	content, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Error reading body, %s", err)
	}

	queue <- Entry{Name: name, Content: content}
	log.Printf("Queued file %s with size %d", name, len(content))
}

func main() {
	if godotenv.Load() != nil {
		log.Println("Error loading .env file")
	}

	go push(os.Getenv("API_KEY"), os.Getenv("ENDPOINT"), os.Getenv("ALBUM"))
	http.HandleFunc("/upload", upload)
	log.Println("Listening on :14994")
	err := http.ListenAndServe("localhost:14994", nil)
	if err != nil {
		return
	} else {
		log.Fatalf("%s", err)
	}
}
