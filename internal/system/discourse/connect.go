package discourse

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/mrusme/neonmodem/internal/browser"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/discourse/api"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/prompt"
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

	if err := browser.Open(openURL, "", sys.logger); err != nil {
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
	if err := sys.verifyKey(ctx, sysURL, clientID, userAPIKey.Key); err != nil {
		return settings, err
	}
	p.Notice(system.HostTitle(sysURL) + " accepted the key.")

	key, err := p.Generated(ctx, prompt.Field{Name: "user API key", Secret: true}, userAPIKey.Key)
	if err != nil {
		return settings, err
	}

	settings.SetCredential(system.CredentialUsername, username)
	settings.SetCredential(system.CredentialKey, key)
	settings.SetCredential(system.CredentialClientID, prompt.Answer{Value: clientID})

	return settings, nil
}

func (sys *System) verifyKey(ctx context.Context, sysURL string, clientID string, key string) error {
	httpClient := httpx.NewHTTPClient(httpx.Options{Proxy: sys.proxy, Logger: sys.logger})
	client, err := api.NewClient(httpClient, sysURL, api.Credentials{ClientID: clientID, Key: key})
	if err != nil {
		return err
	}

	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()
	_, err = client.CurrentSession(bounded)
	err = system.Timeout(ctx, sys.readTimeout, err)

	switch status := httpx.StatusOf(err); {
	case err == nil:
		return nil
	case status == http.StatusForbidden || status == http.StatusNotFound:
		return fmt.Errorf("%s rejected the user API key", system.HostTitle(sysURL))
	default:
		return fmt.Errorf("could not check the user API key with %s: %w", sysURL, err)
	}
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
		return userAPIKey, errors.New("the pasted key was issued for a different request")
	}

	return userAPIKey, nil
}
