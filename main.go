package main

import (
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"encoding/json"
	"log"
	"strings"
	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
	"github.com/trolioSFG/internal/database"
	"database/sql"
//	"context"
	"time"
	"github.com/google/uuid"
	"github.com/trolioSFG/go-server/internal/auth"
)


type apiConfig struct {
	fileserverHits atomic.Int32
	dbq *database.Queries
	platform string
}

type jChirp struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"` 
	UpdatedAt time.Time `json:"updated_at"`
	Body string `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}


// MIDDLEWARE, ¿ always ? return <func...>
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fmt.Println("middleware")
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}


func ready(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte("OK"))
}

func (c *apiConfig) getHits(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset:utf-8")
	w.WriteHeader(http.StatusOK)
	content := []byte(fmt.Sprintf(`<html>
<body>
<h1>Welcome, Chirpy Admin</h1>
<p>Chirpy has been visited %d times!</p>
</body>
</html>
`, c.fileserverHits.Load()))

	w.Write(content)
}

func (c *apiConfig) reset(w http.ResponseWriter, req *http.Request) {
	if c.platform != "dev" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		// c.fileserverHits.Store(0)
		w.Write([]byte(`{"error":"Forbidden"}`))
		return
	}

	c.fileserverHits.Store(0)
	num, err := c.dbq.DeleteUsers(req.Context())
	if err != nil {
		w.Header().Set("Contenty-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"deleted":"%v"}`, num)))
}

func (c *apiConfig) createChirp(w http.ResponseWriter, req *http.Request) {
	type Petition struct {
		Body string `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	pet := Petition{}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&pet)
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Something went wrong"}`))
		return
	}

	log.Printf("New chirp Len(pet.Body) %d\n", len(pet.Body))

	if len(pet.Body) > 140 {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Chirp is too long"}`))
		return
	}

	clean := ""
	for _, word := range strings.Fields(pet.Body) {
		if strings.ToLower(word) == "kerfuffle" ||
			strings.ToLower(word) == "sharbert" ||
			strings.ToLower(word) == "fornax" {
				clean = clean + "**** "
		} else {
			clean = clean + word + " "
		}
	}

	clean = strings.TrimSpace(clean)
	chirp := database.CreateChirpParams {
		Body: clean,
		UserID: pet.UserID,
	}

	created, err := c.dbq.CreateChirp(req.Context(), chirp)
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
		return
	}

	type cchirp struct {
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body string `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	data := cchirp {
		ID: created.ID,
		CreatedAt: created.CreatedAt,
		UpdatedAt: created.UpdatedAt,
		Body: created.Body,
		UserID: created.UserID,
	}
	jData, err := json.Marshal(&data)
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
		return
	}
	

	// No "body" in req json means valid ?!
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(jData)
}


func (c *apiConfig) createUser(w http.ResponseWriter, req *http.Request) {
	type userReq struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}
	data := userReq{}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&data)
	if err != nil {
		/**
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error decoding email"))
		**/
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	hashed, err := auth.HashPassword(data.Password)
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	user, err := c.dbq.CreateUser(req.Context(), database.CreateUserParams {
		Email: data.Email, HashedPassword: hashed })

	if err != nil {
		/**
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Error creating user: %v", err)))
		**/
		if strings.Contains(err.Error(), "duplicate key value") {
			responseWithError(w, http.StatusBadRequest, fmt.Errorf("User already exists"))
		} else {
			responseWithError(w, http.StatusInternalServerError, err)
		}
		return
	}

	// We DO NOT return the hashed password
	// => No need for struct field 
	type udata struct{
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email string `json:"email"`
	}
	uc := udata {
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
	}

	body, err := json.Marshal(&uc)
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(body)
}

func (c *apiConfig) login(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	type userReq struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}
	data := userReq{}
	err := decoder.Decode(&data)
	if err != nil {
		responseWithError(w, http.StatusBadRequest, err)
		return
	}

	dbUser, err := c.dbq.GetUserByEmail(r.Context(), data.Email)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, fmt.Errorf("incorrect email or password"))
		return
	}

	err = auth.CheckPasswordHash(data.Password, dbUser.HashedPassword)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, fmt.Errorf("incorrect email or password"))
		return
	}

	w.Header().Add("Contenty-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"id":"%v","created_at":"%v","updated_at":"%v","email":"%v"}`,
		dbUser.ID, dbUser.CreatedAt, dbUser.UpdatedAt, dbUser.Email)))
}



func responseWithError(w http.ResponseWriter, code int, err error) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
}


func (c *apiConfig) getChirps(w http.ResponseWriter, req *http.Request) {
	jchirps := []jChirp{}

	chirps, err := c.dbq.GetChirps(req.Context())
	if err != nil {
		responseWithError(w, http.StatusInternalServerError,err)
		return
	}
	for _, item := range chirps {
		jchirps = append(jchirps, jChirp{
			ID: item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
			Body: item.Body,
			UserID: item.UserID,
		})
	}

	data, err := json.Marshal(&jchirps)
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *apiConfig) getChirp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	log.Printf("Searching chirp with id: %v", id)
	if id == "" {
		responseWithError(w, http.StatusBadRequest, fmt.Errorf("Missing <id> in request"))
		return
	}

	chirpID, err := uuid.Parse(id)
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	chirp, err := c.dbq.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			responseWithError(w, http.StatusNotFound, err)
		} else {
			responseWithError(w, http.StatusInternalServerError, err)
		}
		return
	}

	jc := jChirp{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	}
	data, err := json.Marshal(&jc)
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Could not connect to DB")
		os.Exit(1)
	}
	dbQueries := database.New(db)

	cfg := apiConfig{
		fileserverHits: atomic.Int32{},
		dbq: dbQueries,
		platform: platform,
	}
	cfg.fileserverHits.Store(0)

	srv := http.NewServeMux()
	srv.HandleFunc("GET /api/healthz", ready)
	srv.HandleFunc("POST /api/chirps", cfg.createChirp)
	srv.HandleFunc("GET /api/chirps", cfg.getChirps)
	srv.HandleFunc("GET /api/chirps/{id}", cfg.getChirp)

	srv.HandleFunc("POST /api/users", cfg.createUser)
	srv.HandleFunc("POST /api/login", cfg.login)

	srv.HandleFunc("GET /admin/metrics", cfg.getHits)
	srv.HandleFunc("POST /admin/reset", cfg.reset)
	srv.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", 
		http.FileServer(http.Dir(".")))))

	s := http.Server{
		Addr: ":8080",
		Handler: srv,
	}

	fmt.Println("Server ready...")
	s.ListenAndServe()
}

