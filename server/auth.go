package server

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"online-calling/debug"
	"time"
)

type HashedPassword struct {
	hashedPassword []byte
	salt           Salt
}

type AuthToken struct {
	Auth    string    `json:"auth_token"`
	Exp     time.Time `json:"expiry"`
	Created time.Time `json:"created"`
}

func NewAuthToken() *AuthToken {
	created := time.Now()
	return &AuthToken{
		Created: created,
		Exp:     created.Add(time.Hour * 720),
		Auth:    GenerateAuthKey(),
	}
}

func (au *AuthToken) IsValid() error {
	if len(au.Auth) == 0 {
		return errors.New("undefined authentication key")
	}
	if au.Created.Compare(au.Exp) > -1 {
		return errors.New("expired authentication")
	}
	return nil
}

const SaltLen = 32

type Salt [SaltLen]byte

func GenerateAuthKey() string {
	return uuid.NewString()
}

func NewSalt() (Salt, error) {
	var b Salt
	_, err := rand.Read(b[:])
	if err != nil {
		return Salt{}, err
	}
	return b, nil
}

func (u *User) ValidatePasswordHash(password string) (*AuthToken, error) {
	if len(u.hashedPassword) == 0 {
		return nil, fmt.Errorf("hashed password for user `%s` is not set", u.Username)
	}
	err := bcrypt.CompareHashAndPassword(u.hashedPassword, []byte(password))
	if err == nil {
		if u.AuthToken.IsValid() == nil {
			return u.AuthToken, nil
		}
		return NewAuthToken(), nil
	}
	return nil, err
}

func hashPassword(pwd string, d ...*debug.Debugger) *HashedPassword {
	salt, err := NewSalt()
	if err != nil {
		if len(d) != 0 && d[0] != nil {
			d[0].DebugPrintln("failed to generate salt for user", err)
		}
		return nil
	}
	hash, _ := bcrypt.GenerateFromPassword(append([]byte(pwd), salt[:]...), 0)
	return &HashedPassword{hash, salt}
}
