package model

import (
	"fmt"
	"regexp"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidateID(id string) error {
	if !safeIDPattern.MatchString(id) {
		return fmt.Errorf("unsafe id %q", id)
	}
	return nil
}
