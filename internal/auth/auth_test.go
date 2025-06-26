package auth

import (
	"testing"
	"github.com/google/uuid"
	"time"
//	"fmt"
)

func TestEqual(t *testing.T) {
	password := "123456"
	hashed, err := HashPassword(password)
	if err != nil {
		t.Errorf("Error calling HashPassword: %v", err)
	}

	err = CheckPasswordHash(password, hashed)
	if err != nil {
		t.Errorf(`password "%s" hash does not match hashed "%s"`, password, hashed)
	}
}

func TestJWT(t *testing.T) {
	id := uuid.New()
	secret := "MySecretKey"
	expires, err := time.ParseDuration("72h")

	token, err := MakeJWT(id, secret, expires)
	if err != nil {
		t.Errorf("Error making JWT: %v", err)
	}

	t.Log(token)

	altid, err := ValidateJWT(token, secret)
	if err != nil {
		t.Errorf("Error validating (unexpected): %v", err)
	}

	if altid != id {
		t.Errorf("UUID do not match!!!")
	}

	t.Log(id, "==", altid)

	// Validate with incorrect secret, expected to fail
	altid, err = ValidateJWT(token, "falseSecret")
	if err == nil {
		t.Errorf("Error validating with erroneous secret!")
	}
	otherToken, err := MakeJWT(id, "falseSecret", expires)
	if err != nil {
		t.Errorf("Error making JWT: %v", err)
	}

	altid, err = ValidateJWT(otherToken, secret)
	if err == nil {
		t.Errorf("Valid token with different secret!")
	}

}

