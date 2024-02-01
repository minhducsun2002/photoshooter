package main

import (
	"bytes"
	"encoding/json"
	"github.com/joho/godotenv"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
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

func push(apiKey string, endpoint string) {
	for {
		l := len(queue)
		if l == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		entry := <-queue

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", entry.Name)
		_, _ = part.Write(entry.Content)

		_ = writer.Close()

		req, _ := http.NewRequest("POST", endpoint, body)
		req.Header.Add("X-API-Key", apiKey)
		req.Header.Add("albumuuid", "")
		req.Header.Add("Content-Type", writer.FormDataContentType())

		response, err := client.Do(req)
		if err != nil {
			log.Printf("Error uploading file %s : %s", entry.Name, err)
			continue
		}

		if response.StatusCode != 200 {
			log.Printf("Error uploading file %s : Response status was %d", entry.Name, response.StatusCode)
			continue
		}

		r := new(UploadResponse)
		err = json.NewDecoder(response.Body).Decode(r)
		if err != nil {
			log.Printf("Error unmarshalling response : %s", err)
			continue
		}

		log.Printf("Uploaded as %s", r.Name)
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

	req.Header.Add("albumuuid", "")
	req.Header.Add("Accept", "*/*")
}

func main() {
	if godotenv.Load() != nil {
		log.Println("Error loading .env file")
	}

	go push(os.Getenv("API_KEY"), os.Getenv("ENDPOINT"))
	http.HandleFunc("/upload", upload)
	log.Println("Listening on :14994")
	err := http.ListenAndServe("localhost:14994", nil)
	if err != nil {
		return
	} else {
		log.Fatalf("%s", err)
	}
}
