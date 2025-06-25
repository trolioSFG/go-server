module github.com/trolioSFG/go-server

go 1.23.2

replace github.com/trolioSFG/internal/database => ./internal/database

require (
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	github.com/trolioSFG/internal/database v0.0.0-00010101000000-000000000000
)

require github.com/google/uuid v1.6.0 // indirect
