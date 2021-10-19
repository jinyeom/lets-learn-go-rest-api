package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Coaster struct {
	Name         string `json:"name,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	ID           string `json:"id,omitempty"`
	InPark       string `json:"inPark,omitempty"`
	Height       int    `json:"height,omitempty"`
}

type coastersHandler struct {
	sync.Mutex
	store map[string]Coaster
}

func newCoastersHandler() *coastersHandler {
	return &coastersHandler{
		store: make(map[string]Coaster),
	}
}

func (ch *coastersHandler) getRandomCoaster(w http.ResponseWriter, r *http.Request) {
	ids := make([]string, 0, len(ch.store))
	ch.Lock()
	for k, _ := range ch.store {
		ids = append(ids, k)
	}
	ch.Unlock()

	if len(ids) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	rand.Seed(time.Now().UnixNano())
	id := ids[rand.Intn(len(ids))]

	w.Header().Add("location", fmt.Sprintf("/coasters/%s", id))
	w.WriteHeader(http.StatusFound)
}

func (ch *coastersHandler) getCoaster(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	id := parts[2]
	if id == "random" {
		// NOTE: this is not necessary...
		ch.getRandomCoaster(w, r)
		return
	}

	ch.Lock()
	coaster, ok := ch.store[id]
	ch.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := json.Marshal(coaster)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (ch *coastersHandler) coasters(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		ch.get(w, r)
		return
	case "POST":
		ch.post(w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("method not allowed"))
		return
	}
}

func (ch *coastersHandler) get(w http.ResponseWriter, r *http.Request) {
	coasters := make([]Coaster, 0, len(ch.store))
	ch.Lock()
	for _, c := range ch.store {
		coasters = append(coasters, c)
	}
	ch.Unlock()

	data, err := json.Marshal(coasters)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (ch *coastersHandler) post(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	if ct := r.Header.Get("content-type"); ct != "application/json" {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		w.Write([]byte(fmt.Sprintf("expected content-type 'application/json', got '%s'", ct)))
		return
	}

	var coaster Coaster
	if err := json.Unmarshal(data, &coaster); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	coaster.ID = fmt.Sprintf("%d", time.Now().UnixNano())

	ch.Lock()
	ch.store[coaster.ID] = coaster
	ch.Unlock()
}

type adminPortal struct {
	password string
}

func newAdminPortal() *adminPortal {
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		log.Fatal("required environment variable ADMIN_PASSWORD not set")
	}
	return &adminPortal{password: password}
}

func (ap *adminPortal) handler(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || username != "admin" || password != ap.password {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
		return
	}
	w.Write([]byte("Welcome admin."))
}

func main() {
	admin := newAdminPortal()
	ch := newCoastersHandler()

	http.HandleFunc("/admin", admin.handler)
	http.HandleFunc("/coasters", ch.coasters)
	http.HandleFunc("/coasters/", ch.getCoaster)
	http.HandleFunc("/coasters/random", ch.getCoaster)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
