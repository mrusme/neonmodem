package discourse

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/system/adapter"
	"github.com/pkg/browser"
	"go.uber.org/zap"
)

type System struct {
	config map[string]interface{}
	logger *zap.SugaredLogger
}

type UserAPIKey struct {
	Key   string `json:"key"`
	Nonce string `json:"nonce"`
	Push  bool   `json:"push"`
	API   int    `json:"api"`
}

func (sys *System) GetConfig() map[string]interface{} {
	return sys.config
}

func (sys *System) SetConfig(cfg *map[string]interface{}) {
	sys.config = *cfg
}

func (sys *System) SetLogger(logger *zap.SugaredLogger) {
	sys.logger = logger
}

func (sys *System) Load() error {

	return nil
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
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes}))

	// Public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes}))

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
	values.Set("scopes", "read,write,notifications")
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

	decryptedUserAPIKey, err := privateKey.Decrypt(
		rand.Reader,
		decodedUserAPIKey,
		nil,
	)
	if err != nil {
		return err
	}

	var userAPIKey UserAPIKey
	err = json.Unmarshal(decryptedUserAPIKey, &userAPIKey)
	if err != nil {
		return err
	}

	// Credentials
	credentials := make(map[string]string)
	credentials["pk"] = privateKeyPEM
	credentials["username"] = username
	credentials["key"] = userAPIKey.Key

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["url"] = sysURL
	sys.config["credentials"] = credentials

	return nil
}

func (sys *System) ListPosts() ([]post.Post, error) {
	credentials := make(map[string]string)
	for k, v := range (sys.config["credentials"]).(map[string]interface{}) {
		credentials[k] = v.(string)
	}
	c := NewClient(&ClientConfig{
		Endpoint:    sys.config["url"].(string),
		Credentials: credentials,
		HTTPClient:  http.DefaultClient,
		Logger:      sys.logger,
	})

	posts, err := c.Posts.List(context.Background())
	if err != nil {
		return []post.Post{}, err
	}

	fmt.Printf("%v\n", posts)

	return []post.Post{}, nil
}

func (sys *System) GetCapabilities() []adapter.Capability {
	var caps []adapter.Capability

	caps = append(caps, adapter.Capability{
		ID:   "posts",
		Name: "Posts",
	})
	caps = append(caps, adapter.Capability{
		ID:   "groups",
		Name: "Groups",
	})
	caps = append(caps, adapter.Capability{
		ID:   "search",
		Name: "Search",
	})

	return caps
}
