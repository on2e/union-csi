package driver

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func parseEndpoint(endpoint string) (scheme string, address string, err error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse endpoint %s: %v", endpoint, err)
	}

	address = filepath.Join(u.Host, filepath.FromSlash(u.Path))
	scheme = strings.ToLower(u.Scheme)

	switch scheme {
	case "tcp":
	case "unix":
		address = filepath.Join("/", address)
		if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("failed to remove unix domain socket %s: %v", address, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, address, nil
}
