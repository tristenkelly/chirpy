package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"

	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	msg := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", cfg.fileserverHits.Load())
	w.Write([]byte(msg))
}

func (cfg *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
}

func validChirp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type paramaters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := paramaters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	type cleanedBody struct {
		Cleaned_Body string `json:"cleaned_body"`
	}

	type returnValsValid struct {
		Valid bool `json:"valid"`
	}

	type errorResponse struct {
		Error string `json:"error"`
	}

	respBodyValid := returnValsValid{
		Valid: false,
	}

	respError := errorResponse{
		Error: "",
	}

	cleanBody := cleanedBody{
		Cleaned_Body: filterText(params.Body),
	}

	if len(params.Body) <= 140 {
		respBodyValid.Valid = true
	} else if len(params.Body) > 140 {
		respError.Error = "Chirp is too long"
	}

	if respBodyValid.Valid {
		val, err := json.Marshal(cleanBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		w.Write(val)
	} else {
		val, err := json.Marshal(respError)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(400)
		w.Write(val)
	}
}

func filterText(text string) string {
	sliceStr := strings.Split(text, " ")
	badWords := map[string]bool{
		"kerfuffle": true,
		"sharbert":  true,
		"fornax":    true,
	}
	for i, str := range sliceStr {
		if badWords[strings.ToLower(str)] {
			sliceStr[i] = "****"
		}
	}
	newString := strings.Join(sliceStr, " ")
	return newString
}

func main() {

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	apiCfg := &apiConfig{}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", health)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)
	mux.HandleFunc("POST /api/validate_chirp", validChirp)

	err := http.ListenAndServe(server.Addr, server.Handler)
	if err != nil {
		log.Fatal("error starting server")
	}
}
