package system

import (
	"context"
	"errors"
	"log/slog"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system/credential"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

type Capabilities uint8

const (
	CapListForums Capabilities = 1 << iota
	CapListPosts
	CapListReplies
	CapCreatePost
	CapCreateReply
)

const (
	CapRead  = CapListForums | CapListPosts | CapListReplies
	CapWrite = CapCreatePost | CapCreateReply
)

const (
	CredentialUsername = "username"
	CredentialPassword = "password"
	CredentialToken    = "token"
	CredentialKey      = "key"
	CredentialClientID = "client_id"
)

func (c Capabilities) Has(want Capabilities) bool {
	return c&want == want
}

type Settings struct {
	URL         string            `toml:"url,omitempty"`
	Credentials map[string]string `toml:"credentials,omitempty"`
	Options     map[string]string `toml:"options,omitempty"`
}

func (s Settings) Credential(key string) string {
	return s.Credentials[key]
}

func (s *Settings) SetCredential(key string, a prompt.Answer) {
	if s.Credentials == nil {
		s.Credentials = map[string]string{}
	}

	if a.Command != "" {
		delete(s.Credentials, key)
		s.Credentials[key+credential.Suffix] = a.Command
		return
	}

	delete(s.Credentials, key+credential.Suffix)
	s.Credentials[key] = a.Value
}

func (s Settings) Option(key string, fallback string) string {
	if v, ok := s.Options[key]; ok && v != "" {
		return v
	}
	return fallback
}

type Env struct {
	Index    int
	Settings Settings
	Proxy    string
	Logger   *slog.Logger
}

func (e Env) Log() *slog.Logger {
	if e.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return e.Logger
}

type System interface {
	Kind() string
	Title() string
	URL() string
	Description() string
	Capabilities() Capabilities

	Connect(ctx context.Context, p prompt.Prompter, sysURL string) (Settings, error)

	ListForums(ctx context.Context) ([]forum.Forum, error)
	Orders(forumID string) Ordering
	ListPosts(ctx context.Context, forumID string, order Order) ([]post.Post, error)
	LoadPost(ctx context.Context, p *post.Post) error
	CreatePost(ctx context.Context, p *post.Post) error
	CreateReply(ctx context.Context, r *reply.Reply) error
}

var (
	ErrUnsupported      = errors.New("this system doesn't support that")
	ErrOrderUnavailable = errors.New("this order isn't available on this site")
	ErrNoCredentials    = errors.New(
		"this system is connected without an account; run neonmodem connect " +
			"again with credentials to post")
	ErrNeedsConnect = errors.New("this system needs neonmodem connect")
)

type connectError struct {
	text string
}

func (e connectError) Error() string {
	return e.text
}

func (e connectError) Unwrap() error {
	return ErrNeedsConnect
}

func NeedsConnect(text string) error {
	return connectError{text: text}
}

func ConnectCommand(kind string, sysURL string) string {
	if sysURL == "" {
		return "neonmodem connect --type " + kind
	}
	return "neonmodem connect --type " + kind + " --url " + sysURL
}
