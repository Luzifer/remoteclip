package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Luzifer/rconfig"
	"github.com/atotto/clipboard"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	cfg = struct {
		Listen         string `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	cache     = []string{}
	cacheLock sync.RWMutex
	version   = "dev"
)

func init() {
	if err := rconfig.Parse(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("git-changerelease %s\n", version)
		os.Exit(0)
	}
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/api/get", handleGet)
	r.HandleFunc("/api/list", handleList)
	r.HandleFunc("/api/set", handleSet)

	go fetchTicker()

	http.ListenAndServe(cfg.Listen, r)
}

func fetchTicker() {
	for range time.Tick(250 * time.Millisecond) {
		content, err := clipboard.ReadAll()
		if err != nil {
			log.WithError(err).Errorf("Failed to fetch clipboard content")
			continue
		}

		cacheLock.Lock()
		if len(cache) > 0 && content == cache[0] {
			cacheLock.Unlock()
			continue
		}
		cache = append([]string{content}, cache...)
		if len(cache) > 100 {
			cache = cache[0:100]
		}
		cacheLock.Unlock()
	}
}

func handleGet(res http.ResponseWriter, r *http.Request) {
	cacheLock.RLock()
	defer cacheLock.RUnlock()
	content := cache[0]

	switch r.Header.Get("Accept") {
	case "application/json":
		res.Header().Set("Content-Type", "application/json")
		json.NewEncoder(res).Encode(map[string]string{
			"content": content,
		})
	default:
		res.Header().Set("Content-Type", http.DetectContentType([]byte(content)))
		res.Write([]byte(content))
	}
}

func handleList(res http.ResponseWriter, r *http.Request) {
	cacheLock.RLock()
	defer cacheLock.RUnlock()

	res.Header().Set("Content-Type", "application/json")
	json.NewEncoder(res).Encode(cache)
}

func handleSet(res http.ResponseWriter, r *http.Request) {
	var content string

	switch r.Header.Get("Content-Type") {
	case "application/json":
		fallthrough
	case "text/json":
		c := map[string]string{}
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			log.WithError(err).Errorf("Unable to decode input json")
			http.Error(res, "Was not able to parse input JSON", http.StatusBadRequest)
			return
		}
		if cont, ok := c["content"]; ok {
			content = cont
		} else {
			http.Error(res, "JSON needs to contain key 'content'", http.StatusBadRequest)
			return
		}

	default:
		content = r.FormValue("content")
	}

	if content != "" {
		if err := clipboard.WriteAll(content); err != nil {
			log.WithError(err).Errorf("Unable to set clipboard")
			http.Error(res, "Was not able to set clipboard", http.StatusInternalServerError)
			return
		}
	}
}
