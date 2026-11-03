package main

import (
	"errors"
	"net/url"
	"strings"
)

func normalizePublicBaseURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("public base URL is malformed")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("public base URL must use HTTP or HTTPS")
	}

	if parsed.Hostname() == "" {
		return "", errors.New("public base URL must include a hostname")
	}

	if parsed.User != nil {
		return "", errors.New("public base URL must not include credentials")
	}

	if parsed.RawQuery != "" || parsed.ForceQuery {
		return "", errors.New("public base URL must not include a query")
	}

	if strings.Contains(rawURL, "#") {
		return "", errors.New("public base URL must not include a fragment")
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("public base URL must not include a path prefix")
	}

	parsed.Path = ""
	parsed.RawPath = ""

	return parsed.String(), nil
}
