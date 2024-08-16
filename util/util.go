package util

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 50
	minUsernameLength = 2
	maxUsernameLength = 16
	specialChars      = "!@#$%^&*()_+"
)

func IsUpper[T rune | string](t T) bool {
	return strings.ToUpper(string(t)) == string(t)
}

func IsLower[T rune | string](t T) bool {
	return strings.ToLower(string(t)) == string(t)
}

func StringsCountFunc(s string, pred func(r rune) bool) int {
	var count int
	for _, r := range s {
		if pred(r) {
			count++
		}
	}
	return count
}

func IsValidPassword(password string) error {
	if err := validPasswordLength(password); err != nil {
		return err
	}
	return passwordContains(password)
}

func IsValidUsername(username string) error {
	return inLengthRange(username, minUsernameLength, maxUsernameLength)
}

func validUsernameLength(username string) error {
	return inLengthRange(username, minUsernameLength, maxPasswordLength)
}

func validPasswordLength(password string) error {
	return inLengthRange(password, minPasswordLength, maxPasswordLength)
}

func inLengthRange(s string, min int, max int) error {
	if len(s) < min {
		return fmt.Errorf("password length is less than %d characters", min)
	}
	if len(s) > max {
		return fmt.Errorf("password length is over %d characters", max)
	}
	return nil
}

func passwordContains(password string) error {
	if !strings.ContainsFunc(password, IsLower[rune]) {
		return errors.New("password must contain at least one lowercase character")
	}
	if !strings.ContainsFunc(password, IsUpper[rune]) {
		return errors.New("password must contain at least one uppercase character")
	}
	if !strings.ContainsFunc(password, func(r rune) bool { return strings.Contains(specialChars, string(r)) }) {
		return errors.New("password must contain at least one number")
	}
	if !strings.ContainsFunc(password, func(r rune) bool { _, err := strconv.Atoi(string(r)); return err == nil }) {
		return errors.New("password must contain at least one special character")
	}
	return nil
}
