package discourse

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/pkg/browser"
)

type UserAPIKey struct {
	Key   string `json:"key"`
	Nonce string `json:"nonce"`
	Push  bool   `json:"push"`
	API   int    `json:"api"`
}

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: sysURL}

	username, err := p.Credential(ctx, prompt.Field{
		Name:      "username",
		Question:  "Please enter your username",
		NoAccount: true,
	})
	if err != nil {
		return settings, err
	}
	if username.NoAccount {
		p.Notice("Connecting without an account; posting and replying will not be available.")
		return settings, nil
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return settings, err
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return settings, err
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}))

	clientID := uuid.NewString()

	nonceBytes := make([]byte, 20)
	if _, err := rand.Read(nonceBytes); err != nil {
		return settings, err
	}
	nonce := base64.URLEncoding.EncodeToString(nonceBytes)

	values := url.Values{}
	values.Set("application_name", "neonmodem")
	values.Set("client_id", clientID)
	values.Set("scopes", "read,write")
	values.Set("public_key", publicKeyPEM)
	values.Set("nonce", nonce)
	openURL := fmt.Sprintf("%s/user-api-key/new?%s", sysURL, values.Encode())

	browser.Stdout = nil
	browser.Stderr = nil
	if err := browser.OpenURL(openURL); err != nil {
		p.Notice("Could not open a browser. Please open this address yourself:")
		p.Notice(openURL)
	}

	encodedUserAPIKey, err := p.Line(
		"\nPlease copy the user API key after authorizing and paste it here",
		"user API key",
	)
	if err != nil {
		return settings, err
	}

	userAPIKey, err := decodeUserAPIKey(privateKey, encodedUserAPIKey, nonce)
	if err != nil {
		return settings, err
	}

	key, err := p.Generated(ctx, prompt.Field{Name: "user API key", Secret: true}, userAPIKey.Key)
	if err != nil {
		return settings, err
	}

	settings.SetCredential(system.CredentialUsername, username)
	settings.SetCredential(system.CredentialKey, key)
	settings.SetCredential(system.CredentialClientID, prompt.Answer{Value: clientID})

	return settings, nil
}

func decodeUserAPIKey(
	privateKey *rsa.PrivateKey,
	pasted string,
	nonce string,
) (UserAPIKey, error) {
	var userAPIKey UserAPIKey

	encoded := strings.Join(strings.Fields(pasted), "")

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return userAPIKey, fmt.Errorf("the pasted key is not valid base64: %w", err)
	}

	plaintext, err := privateKey.Decrypt(rand.Reader, ciphertext, nil)
	if err != nil {
		return userAPIKey, fmt.Errorf("the pasted key could not be decrypted: %w", err)
	}

	if err := json.Unmarshal(plaintext, &userAPIKey); err != nil {
		return userAPIKey, fmt.Errorf("the pasted key has an unexpected format: %w", err)
	}
	if userAPIKey.Nonce != nonce {
		return userAPIKey, fmt.Errorf("the pasted key was issued for a different request")
	}

	return userAPIKey, nil
}
