package discourse

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/browser"
)

type UserAPIKey struct {
	Key   string `json:"key"`
	Nonce string `json:"nonce"`
	Push  bool   `json:"push"`
	API   int    `json:"api"`
}

func (sys *System) Connect(sysURL string) error {
	var err error

	// Request input from user
	scanner := bufio.NewScanner(os.Stdin)
	var username string = ""
	for username == "" {
		fmt.Printf(
			"Please enter your username: ",
		)
		scanner.Scan()
		username = strings.ReplaceAll(scanner.Text(), " ", "")
		if username == "" {
			fmt.Println("Invalid input")
		}
	}

	// Private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}))

	// Public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}))

	// Client ID
	uuidV4, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	clientID := uuidV4.String()

	// Nonce
	randomBytes := make([]byte, 20)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}
	nonce := base64.URLEncoding.EncodeToString(randomBytes)

	// URL
	baseURL := fmt.Sprintf("%s/user-api-key/new", sysURL)
	values := url.Values{}
	values.Set("application_name", "gobbs")
	values.Set("client_id", clientID)
	values.Set("scopes", "read,write")
	values.Set("public_key", publicKeyPEM)
	values.Set("nonce", nonce)

	// Open in browser
	openURL := baseURL + "?" + values.Encode()
	if err := browser.OpenURL(openURL); err != nil {
		return err
	}

	// Request input from user
	scanner = bufio.NewScanner(os.Stdin)
	var encodedUserAPIKey string = ""
	for encodedUserAPIKey == "" {
		fmt.Printf(
			"\nPlease copy the user API key after authorizing and paste it here: ",
		)
		scanner.Scan()
		encodedUserAPIKey = strings.ReplaceAll(scanner.Text(), " ", "")
		if encodedUserAPIKey == "" {
			fmt.Println("Invalid input")
		}
	}

	// API key
	decodedUserAPIKey, err := base64.StdEncoding.DecodeString(encodedUserAPIKey)
	if err != nil {
		return err
	}
	fmt.Println(string(decodedUserAPIKey))

	decryptedUserAPIKey, err := privateKey.Decrypt(
		rand.Reader,
		decodedUserAPIKey,
		nil,
	)
	if err != nil {
		return err
	}
	fmt.Println(string(decryptedUserAPIKey))

	var userAPIKey UserAPIKey
	err = json.Unmarshal(decryptedUserAPIKey, &userAPIKey)
	if err != nil {
		return err
	}
	fmt.Printf("%v\n", userAPIKey)

	// Credentials
	credentials := make(map[string]string)
	credentials["pk"] = privateKeyPEM
	credentials["username"] = username
	credentials["key"] = userAPIKey.Key
	credentials["client_id"] = clientID

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["url"] = sysURL
	sys.config["credentials"] = credentials

	return nil
}
