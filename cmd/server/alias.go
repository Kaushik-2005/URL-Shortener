package main

import (
	"errors"
	"strings"
)

func validateAlias(alias string) error {
	if len(alias) < 3 || len(alias) > 32 {
		return errors.New("alias must contain between 3 and 32 characters")
	}

	for _, character := range alias {
		allowed := (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' ||
			character == '_'

		if !allowed {
			return errors.New(
				"alias may contain only ASCII letters, digits, hyphens, and underscores",
			)
		}
	}

	switch strings.ToLower(alias) {
	case "healthz", "readyz", "api":
		return errors.New("alias is reserved")
	}

	return nil
}
