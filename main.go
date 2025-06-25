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
)


type apiConfig struct {
	fileserverHits atomic.Int32
	dbq *database.Queries
	platform string
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

func validate(w http.ResponseWriter, req *http.Request) {
	type Petition struct {
		Body string `json:"body"`
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

	log.Printf("Len(pet.Body) %d\n", len(pet.Body))

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

			
	// No "body" in req json means valid ?!
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"cleaned_body":"` + clean + `"}`))
}


func (c *apiConfig) createUser(w http.ResponseWriter, req *http.Request) {
	type userReq struct {
		Email string `json:"email"`
	}
	data := userReq{}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&data)
	if err != nil {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error decoding email"))
		return
	}

	user, err := c.dbq.CreateUser(req.Context(), data.Email)
	if err != nil {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Error creating user: %v", err)))
		return
	}

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
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error decoding email"))
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(body)
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
	srv.HandleFunc("POST /api/validate_chirp", validate)
	srv.HandleFunc("POST /api/users", cfg.createUser)

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

