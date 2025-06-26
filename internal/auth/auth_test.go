package auth

import (
	"testing"
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

