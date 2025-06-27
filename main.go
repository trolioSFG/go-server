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
//	"strconv"
)


type apiConfig struct {
	fileserverHits atomic.Int32
	dbq *database.Queries
	platform string
	secret string
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
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	fmt.Println("Received token:", token)

	id, err := auth.ValidateJWT(token, c.secret)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}
	fmt.Println("CreateChirp JWT userID:", id)

	type Petition struct {
		Body string `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	pet := Petition{}

	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(&pet)
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

	// Use userID from JWT instead of petition
	// UserID: pet.UserID,
	clean = strings.TrimSpace(clean)
	chirp := database.CreateChirpParams {
		Body: clean,
		UserID: id,
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

func (c *apiConfig) updateUser(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	userID, err := auth.ValidateJWT(token, c.secret)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	decoder := json.NewDecoder(r.Body)
	type userReq struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}
	ur := userReq{}
	err = decoder.Decode(&ur)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
	}
	hashed, err := auth.HashPassword(ur.Password)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
	}

	newUser, err := c.dbq.UpdateUser(r.Context(), database.UpdateUserParams{
		ID: userID,
		HashedPassword: hashed,
		Email: ur.Email,
	})
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"id":"%v","email":"%v","created_at":"%v","updated_at":"%v"}`,
		newUser.ID, newUser.Email, newUser.CreatedAt, newUser.UpdatedAt)))

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

	/**
	if data.ExpiresInSeconds == 0 || data.ExpiresInSeconds > 3600 {
		data.ExpiresInSeconds = 3600
	}
	**/

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

	duration, err := time.ParseDuration("1h")
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	token, err := auth.MakeJWT(dbUser.ID, c.secret, duration)
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}

	refToken, _ := auth.MakeRefreshToken()

	p := database.SaveRefreshTokenParams {
		Token: refToken,
		UserID: dbUser.ID,
	}
	err = c.dbq.SaveRefreshToken(r.Context(), p)
	if err != nil {
		responseWithError(w, http.StatusInternalServerError, err)
		return
	}


	w.Header().Add("Contenty-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(
		`{"id":"%v","created_at":"%v","updated_at":"%v","email":"%v","token":"%v","refresh_token":"%v"}`,
		dbUser.ID, dbUser.CreatedAt, dbUser.UpdatedAt, dbUser.Email, token, refToken)))
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

func (c *apiConfig) refresh(w http.ResponseWriter, r *http.Request) {
	rtoken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	data, err := c.dbq.GetRefreshToken(r.Context(), rtoken)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	fmt.Printf("Refresh token:\n%+v\n", data)

	if data.RevokedAt.Valid || time.Now().After(data.ExpiresAt) {
		responseWithError(w, http.StatusUnauthorized, fmt.Errorf("Expired refresh token"))
		return
	}

	/**
	userID, err := c.dqb.GetUserIDFromRefreshToken(r.Context(), rtoken)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}
	**/

	duration, _ := time.ParseDuration("1h")
	token, err := auth.MakeJWT(data.UserID, c.secret, duration)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"token":"%v"}`, token)))
}

func (c *apiConfig) revoke(w http.ResponseWriter, r *http.Request) {
	rtoken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	/** Is it necesary to check si exists token ?
	_, err := c.dbq.GetRefreshToken(r.Context(), rtoken)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}
	***/

	err = c.dbq.UpdateRefreshToken(r.Context(), rtoken)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
	}

	w.WriteHeader(http.StatusNoContent)
}



func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	secret := os.Getenv("SECRET")
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
		secret: secret,
	}
	cfg.fileserverHits.Store(0)

	srv := http.NewServeMux()
	srv.HandleFunc("GET /api/healthz", ready)
	srv.HandleFunc("POST /api/chirps", cfg.createChirp)
	srv.HandleFunc("GET /api/chirps", cfg.getChirps)
	srv.HandleFunc("GET /api/chirps/{id}", cfg.getChirp)

	srv.HandleFunc("POST /api/users", cfg.createUser)
	srv.HandleFunc("PUT /api/users", cfg.updateUser)
	srv.HandleFunc("POST /api/login", cfg.login)
	srv.HandleFunc("POST /api/refresh", cfg.refresh)
	srv.HandleFunc("POST /api/revoke", cfg.revoke)

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

