package discourse

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

var testPrivateKey = sync.OnceValues(func() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
})

func privateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := testPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func wrapAt60(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var b strings.Builder
	for len(encoded) > 60 {
		b.WriteString(encoded[:60])
		b.WriteByte('\n')
		encoded = encoded[60:]
	}
	b.WriteString(encoded)
	b.WriteByte('\n')
	return b.String()
}

func encryptedPayload(t *testing.T, key *rsa.PrivateKey, nonce string) string {
	t.Helper()

	payload, err := json.Marshal(UserAPIKey{Key: "abc123", Nonce: nonce, API: 4})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, &key.PublicKey, payload)
	if err != nil {
		t.Fatal(err)
	}

	return wrapAt60(ciphertext)
}

func TestDecodeUserAPIKeyIgnoresWhitespace(t *testing.T) {
	key := privateKey(t)
	wrapped := encryptedPayload(t, key, "nonce-1")

	spaced := strings.TrimSpace(strings.ReplaceAll(wrapped, "\n", " "))
	if strings.IndexByte(spaced, ' ') != 60 {
		t.Fatalf("expected the first space at byte 60 like a browser copy, got %d",
			strings.IndexByte(spaced, ' '))
	}

	cases := map[string]string{
		"spaces from a browser copy": spaced,
		"line breaks":                wrapped,
		"tabs and carriage returns":  strings.ReplaceAll(wrapped, "\n", "\r\n\t"),
		"surrounding whitespace":     "  " + strings.ReplaceAll(wrapped, "\n", "") + " \n",
	}

	for name, pasted := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodeUserAPIKey(key, pasted, "nonce-1")
			if err != nil {
				t.Fatalf("decodeUserAPIKey: %v", err)
			}
			if got.Key != "abc123" || got.API != 4 {
				t.Errorf("unexpected key: %+v", got)
			}
		})
	}
}

func TestDecodeUserAPIKeyErrors(t *testing.T) {
	key := privateKey(t)

	if _, err := decodeUserAPIKey(key, "not base64!", "nonce-1"); err == nil ||
		!strings.Contains(err.Error(), "not valid base64") {
		t.Errorf("expected a base64 error, got %v", err)
	}

	wrapped := encryptedPayload(t, key, "nonce-1")
	if _, err := decodeUserAPIKey(key, wrapped, "another-nonce"); err == nil ||
		!strings.Contains(err.Error(), "different request") {
		t.Errorf("expected a nonce mismatch error, got %v", err)
	}
}
