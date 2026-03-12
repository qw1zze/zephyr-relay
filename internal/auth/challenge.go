package auth

import "time"

type Challenge struct {
	Nonce     string
	CreatedAt time.Time
}

func (a *Auth) NewChallenge() (string, error) {
	return "", nil
}

func (a *Auth) Verify(nonce, address, signature string) (bool, error) {
	return false, nil
}
