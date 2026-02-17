package config

import (
	"errors"
	"fmt"
	"time"

	gokeyring "github.com/zalando/go-keyring"
)

const keyringTimeout = 3 * time.Second

var errKeyringNotFound = errors.New("secret not found in keyring")

type keyringTimeoutError struct {
	op string
}

func (e *keyringTimeoutError) Error() string {
	return fmt.Sprintf("keyring %s timed out after %v", e.op, keyringTimeout)
}

func keyringSet(service, user, password string) error {
	ch := make(chan error, 1)
	go func() {
		ch <- gokeyring.Set(service, user, password)
	}()
	select {
	case err := <-ch:
		return err
	case <-time.After(keyringTimeout):
		return &keyringTimeoutError{op: "set"}
	}
}

func keyringGet(service, user string) (string, error) {
	type result struct {
		val string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		val, err := gokeyring.Get(service, user)
		ch <- result{val, err}
	}()
	select {
	case r := <-ch:
		if errors.Is(r.err, gokeyring.ErrNotFound) {
			return "", errKeyringNotFound
		}
		return r.val, r.err
	case <-time.After(keyringTimeout):
		return "", &keyringTimeoutError{op: "get"}
	}
}

func keyringDelete(service, user string) error {
	ch := make(chan error, 1)
	go func() {
		ch <- gokeyring.Delete(service, user)
	}()
	select {
	case err := <-ch:
		return err
	case <-time.After(keyringTimeout):
		return &keyringTimeoutError{op: "delete"}
	}
}
