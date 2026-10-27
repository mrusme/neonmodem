package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const nodeinfoPath = "/nodeinfo/2.1"

type nodeinfo struct {
	Software struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"software"`
}

func (c *Client) readVersion(ctx context.Context) (Version, error) {
	var info nodeinfo
	if err := c.site.Get(ctx, nodeinfoPath, nil, &info); err != nil {
		return 0, fmt.Errorf("reading the software version: %w", err)
	}
	return apiVersion(info.Software.Name, info.Software.Version)
}

func apiVersion(name string, version string) (Version, error) {
	switch {
	case name == "":
		return 0, errors.New("the site's nodeinfo doesn't name its software")
	case name != "lemmy":
		return 0, fmt.Errorf("the site runs %s %s, not Lemmy", name, version)
	}

	major, _, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(major)
	switch {
	case err != nil || n < 0:
		return 0, fmt.Errorf("can't tell which API Lemmy %q serves", version)
	case n == 0:
		return V3, nil
	default:
		return V4, nil
	}
}
