package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	password := "correct-horse-battery-staple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash == password {
		t.Fatal("stored hash must not equal plaintext password")
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Fatal("correct password was rejected")
	}

	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Fatal("incorrect password was accepted")
	}
}

func TestPasswordHashesUseDifferentSalts(t *testing.T) {
	password := "same-password"

	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal("same password should produce different stored hashes")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	_, err := VerifyPassword("password", "not-a-valid-hash")
	if err == nil {
		t.Fatal("expected malformed hash to return an error")
	}
}
