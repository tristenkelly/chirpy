package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/tristenkelly/chirpy/internal/auth"
	"github.com/tristenkelly/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
}

type chirpResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
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
	if cfg.platform == "dev" {
		cfg.db.ResetChirps(r.Context())
		cfg.db.ResetUsers(r.Context())
		w.WriteHeader(200)
		w.Write([]byte("reset users\n"))
	} else {
		w.WriteHeader(403)
	}
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type paramters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := paramters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("error decoding json")
		w.WriteHeader(500)
		return
	}

	type userResponse struct {
		Id         uuid.UUID `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Email      string    `json:"email"`
	}
	currentTime := time.Now()
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("error hashing password %v", err)
		w.WriteHeader(500)
		return
	}
	dbParams := database.CreateUserParams{
		ID:             uuid.New(),
		CreatedAt:      currentTime,
		UpdatedAt:      currentTime,
		Email:          params.Email,
		HashedPassword: hashedPassword,
	}

	user, err := cfg.db.CreateUser(r.Context(), dbParams)
	if err != nil {
		log.Printf("error creating user %v", err)
		w.WriteHeader(500)
		return
	}

	returnUser := userResponse{
		Id:         user.ID,
		Created_at: user.CreatedAt,
		Updated_at: user.UpdatedAt,
		Email:      user.Email,
	}

	val, err := json.Marshal(returnUser)
	if err != nil {
		log.Printf("error marshaling json %v", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(201)
	w.Write(val)

}

func (cfg *apiConfig) getChirps(w http.ResponseWriter, r *http.Request) {
	data, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("error getting all chirps %v", err)
	}
	var apiChirp []chirpResponse
	for _, chirp := range data {
		apiChirp = append(apiChirp, chirpResponse{
			ID:        chirp.ID,
			Body:      chirp.Body,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			UserID:    chirp.UserID,
		})
	}
	val, err := json.Marshal(apiChirp)
	if err != nil {
		log.Printf("error marshaling chirp data %v", err)
	}
	w.WriteHeader(200)
	w.Write(val)
}

func (cfg *apiConfig) getChirp(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	chirpUUID, err := uuid.Parse(chirpID)
	if err != nil {
		log.Printf("error converting ID to UUID %v", err)
	}
	data, err := cfg.db.GetChirp(r.Context(), chirpUUID)
	if err != nil {
		log.Printf("error getting chirp %v", err)
		w.WriteHeader(404)
		return
	}
	validChirp := chirpResponse{
		ID:        data.ID,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Body:      data.Body,
		UserID:    data.UserID,
	}
	val, err := json.Marshal(validChirp)
	if err != nil {
		log.Printf("error marshaling chirp data %v", err)
	}
	w.WriteHeader(200)
	w.Write(val)
}

func (cfg *apiConfig) validChirp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type paramaters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
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

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("error getting bearer token %v", err)
		w.WriteHeader(500)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		log.Printf("JWT not valid: %v", err)
		w.WriteHeader(401)
		return
	}

	if respBodyValid.Valid {
		chirpParams := database.CreateChirpParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Body:      cleanBody.Cleaned_Body,
			UserID:    userID,
		}
		chirp, err := cfg.db.CreateChirp(r.Context(), chirpParams)
		if err != nil {
			log.Printf("error creating chirp %v", err)
		}
		validChirpResponse := chirpResponse{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		}
		val, err := json.Marshal(validChirpResponse)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(201)
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

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	type paramaters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	params := paramaters{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("error decoding json for login %v", err)
		w.WriteHeader(500)
		return
	}

	user, err := cfg.db.GetHashedPass(r.Context(), params.Email)
	if err != nil {
		log.Printf("error getting user in login query %v", err)
	}
	token, err := auth.MakeJWT(user.ID, cfg.secret)
	if err != nil {
		log.Printf("error creating JWT %v", err)
		w.WriteHeader(500)
		return
	}
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		log.Printf("error creating refresh token %v", err)
		w.WriteHeader(500)
		return
	}
	revokedAt := sql.NullTime{
		Time:  time.Time{},
		Valid: false,
	}

	rtParams := database.CreateRefreshTokenParams{
		Token:     refreshToken,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour),
		RevokedAt: revokedAt,
	}

	type returnVals struct {
		Id            uuid.UUID `json:"id"`
		Created_at    time.Time `json:"created_at"`
		Updated_at    time.Time `json:"updated_at"`
		Email         string    `json:"email"`
		Token         string    `json:"token"`
		Refresh_token string    `json:"refresh_token"`
	}

	returnuserVals := returnVals{
		Id:            user.ID,
		Created_at:    user.CreatedAt,
		Updated_at:    user.UpdatedAt,
		Email:         user.Email,
		Token:         token,
		Refresh_token: refreshToken,
	}

	err2 := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err2 != nil {
		log.Println("incorrect password")
		w.WriteHeader(401)
		return
	}

	_, err3 := cfg.db.CreateRefreshToken(r.Context(), rtParams)
	if err3 != nil {
		log.Printf("error creating refresh token in table: %v", err)
		w.WriteHeader(500)
		return
	}

	val, err := json.Marshal(returnuserVals)
	if err != nil {
		log.Println("error marshalling login data")
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
	w.Write(val)
}

func (cfg *apiConfig) getRefreshToken(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("error getting bearer token: %v", err)
		w.WriteHeader(500)
		return
	}
	type jwebToken struct {
		Token string `json:"token"`
	}
	rfToken, err := cfg.db.GetResponseToken(r.Context(), token)
	if err != nil {
		log.Printf("error getting refresh token from table: %v", err)
		w.WriteHeader(500)
		return
	}

	if rfToken.ExpiresAt.Before(time.Now()) {
		w.WriteHeader(401)
		return
	}

	if rfToken.RevokedAt.Valid {
		w.WriteHeader(401)
		return
	}

	jwt, err := auth.MakeJWT(rfToken.UserID, cfg.secret)
	if err != nil {
		log.Printf("error making new jwt: %v", err)
		w.WriteHeader(500)
		return
	}
	validToken := jwebToken{
		Token: jwt,
	}
	val, err := json.Marshal(validToken)
	if err != nil {
		log.Printf("error marshalling json data for token: %v", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
	w.Write(val)

}

func (cfg *apiConfig) revokeRefreshToken(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("error getting bearer token: %v", err)
		w.WriteHeader(500)
		return
	}

	revokedAt := sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}

	tokenParams := database.RevokeTokenParams{
		Token:     token,
		RevokedAt: revokedAt,
		UpdatedAt: time.Now(),
	}

	_, err2 := cfg.db.RevokeToken(r.Context(), tokenParams)
	if err2 != nil {
		log.Printf("error updating revoke token field %v", err)
		w.WriteHeader(500)
	}
	w.WriteHeader(204)
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
	envErr := godotenv.Load()
	if envErr != nil {
		log.Fatal("failed to load env variables")
	}
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")
	db, err2 := sql.Open("postgres", dbURL)
	if err2 != nil {
		log.Fatal("error making SQL connection")
	}
	dbQueries := database.New(db)
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	apiCfg := &apiConfig{
		db:       dbQueries,
		platform: platform,
		secret:   secret,
	}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", health)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)
	mux.HandleFunc("POST /api/users", apiCfg.createUser)
	mux.HandleFunc("POST /api/chirps", apiCfg.validChirp)
	mux.HandleFunc("GET /api/chirps", apiCfg.getChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirp)
	mux.HandleFunc("POST /api/login", apiCfg.handleLogin)
	mux.HandleFunc("POST /api/refresh", apiCfg.getRefreshToken)
	mux.HandleFunc("POST /api/revoke", apiCfg.revokeRefreshToken)

	err := http.ListenAndServe(server.Addr, server.Handler)
	if err != nil {
		log.Fatal("error starting server", err)
	}
}
