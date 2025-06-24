package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)


type apiConfig struct {
	fileserverHits atomic.Int32
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
	w.Header().Set("Content-Type", "text/plain;charset:utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hits: %d", c.fileserverHits.Load())))
}

func (c *apiConfig) reset(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain;charset:utf-8")
	w.WriteHeader(http.StatusOK)
	c.fileserverHits.Store(0)
	w.Write([]byte(fmt.Sprintf("Reset: %d", c.fileserverHits.Load())))
}


func main() {
	cfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}
	cfg.fileserverHits.Store(0)

	srv := http.NewServeMux()
	srv.HandleFunc("GET /healthz", ready)
	srv.HandleFunc("GET /metrics", cfg.getHits)
	srv.HandleFunc("POST /reset", cfg.reset)
	srv.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", 
		http.FileServer(http.Dir(".")))))

	s := http.Server{
		Addr: ":8080",
		Handler: srv,
	}

	fmt.Println("Server ready...")
	s.ListenAndServe()
}

