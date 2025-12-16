package security

import "testing"

func TestGenerateOpaqueToken(t *testing.T) {
	token, hash, err := GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("generate token error: %v", err)
	}
	if token == "" {
		t.Fatalf("token is empty")
	}
	if len(hash) == 0 {
		t.Fatalf("hash is empty")
	}

	token2, _, err := GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("generate token2 error: %v", err)
	}
	if token2 == token {
		t.Fatalf("tokens should be different")
	}
}
