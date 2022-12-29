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
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/system/adapter"
	"github.com/pkg/browser"
)

type System struct {
	config map[string]interface{}

	privateKey   *rsa.PrivateKey
	publicKeyPEM string
	userAPIKey   string
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

func (sys *System) Load() error {

	return nil
}

func (sys *System) Login(args map[string]string) error {
	var err error

	sys.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(sys.privateKey.PublicKey)
	if err != nil {
		return err
	}
	sys.publicKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes}))

	uuidV4, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	clientID := uuidV4.String()

	randomBytes := make([]byte, 20)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}
	nonce := base64.URLEncoding.EncodeToString(randomBytes)

	baseURL := fmt.Sprintf("https://%s/user-api-key/new", args["siteURL"])
	values := url.Values{}
	values.Set("application_name", "gobbs")
	values.Set("client_id", clientID)
	values.Set("scopes", "read,write,notifications")
	values.Set("public_key", sys.publicKeyPEM)
	values.Set("nonce", nonce)

	openURL := baseURL + "?" + values.Encode()
	if err := browser.OpenURL(openURL); err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)

	var encodedUserAPIKey string = ""
	for encodedUserAPIKey == "" {
		scanner.Scan()
		encodedUserAPIKey = strings.ReplaceAll(scanner.Text(), " ", "")
		if encodedUserAPIKey == "" {
			fmt.Println("Invalid input")
		}
	}

	textUserAPIKey, err := base64.StdEncoding.DecodeString(encodedUserAPIKey)
	if err != nil {
		return err
	}

	decryptedUserAPIKey, err := sys.privateKey.Decrypt(
		rand.Reader,
		textUserAPIKey,
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

	sys.userAPIKey = userAPIKey.Key

	return nil
}

func (sys *System) ListPosts() ([]post.Post, error) {
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
