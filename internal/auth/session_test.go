package auth

import "testing"

func TestGenerateSessionToken(t *testing.T) {
	first, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken failed: %v", err)
	}

	second, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken failed: %v", err)
	}

	if first == second {
		t.Fatal("generated session tokens should be different")
	}

	if len(first) != 64 {
		t.Fatalf("expected token length 64, got %d", len(first))
	}
}

func TestHashSessionToken(t *testing.T) {
	token := "test-session-token"

	first := HashSessionToken(token)
	second := HashSessionToken(token)

	if first != second {
		t.Fatal("same token should produce the same hash")
	}

	if first == token {
		t.Fatal("hash should not equal the raw token")
	}
}
