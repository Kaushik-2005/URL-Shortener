package main

import (
	"errors"
	"fmt"
	"net/url"
)

func validateURL(rawUrl string) error {
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must use http or https")
	}

	if parsed.Hostname() == "" {
		return errors.New("URL must include a hostname")
	}

	return nil
}
