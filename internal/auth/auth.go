package auth

import (
	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v5"
	"time"
	"github.com/google/uuid"
	"fmt"
	"net/http"
	"strings"
	"log"
	"crypto/rand"
	"encoding/hex"
)


func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 0)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := jwt.NumericDate { time.Now() }
	future := jwt.NumericDate { time.Now().Add(expiresIn) }
	log.Printf("New TOKEN Expires: %v", future)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy",
		IssuedAt: &now,
		ExpiresAt: &future,
		Subject: userID.String(),
})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	// fmt.Println(tokenString, err)
	return tokenString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	// claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		// return hmacSampleSecret, nil
		return []byte(tokenSecret), nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			expired, _ := token.Claims.GetExpirationTime()
			return uuid.UUID{}, fmt.Errorf("Expired %v", expired) 
		}
		return uuid.UUID{}, err
	}

	userID, err := token.Claims.GetSubject()
	// fmt.Println(token.Claims.GetSubject())
	if err != nil {
		return uuid.UUID{}, err
	}

	id, err := uuid.Parse(userID)
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHdr := headers.Get("Authorization")
	if authHdr == "" {
		return "", fmt.Errorf("No auth header")
	}

	token := strings.TrimPrefix(authHdr, "Bearer ")
	if token == authHdr {
		return "", fmt.Errorf("No token")
	}
	return token, nil
}

func MakeRefreshToken() (string, error) {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key), nil
}
	
