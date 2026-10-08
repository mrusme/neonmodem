package system

import (
	"fmt"
	"net/url"
)

func HostTitle(sysURL string) string {
	u, err := url.Parse(sysURL)
	if err != nil || u.Hostname() == "" {
		return sysURL
	}
	return u.Hostname()
}

func Describe(name string, account bool) string {
	if account {
		return name
	}
	return name + " (read-only)"
}

func AccountCapabilities(account bool) Capabilities {
	if account {
		return CapRead | CapWrite
	}
	return CapRead
}

func NoCredentials(kind string, sysURL string) error {
	return fmt.Errorf("%w; run %s again with credentials to post",
		ErrNoCredentials, ConnectCommand(kind, sysURL))
}
