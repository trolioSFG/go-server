module github.com/trolioSFG/go-server

go 1.23.2

replace github.com/trolioSFG/internal/database => ./internal/database

// replace internal/auth => ./internal/auth

require (
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	github.com/trolioSFG/internal/database v0.0.0-00010101000000-000000000000
//	internal/auth v0.0.0-00010101000000-000000000000
)

require github.com/google/uuid v1.6.0

require golang.org/x/crypto v0.39.0

require github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
