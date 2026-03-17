package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Challenge struct {
	Nonce     string
	CreatedAt time.Time
}

func (a *Auth) NewChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(b)

	a.mu.Lock()
	a.challenges[nonce] = Challenge{Nonce: nonce, CreatedAt: time.Now()}
	a.mu.Unlock()

	a.once.Do(func() {
		a.startCleanup(a.ctx)
	})

	return nonce, nil
}

func (a *Auth) Verify(nonce, address, signature string) (bool, error) {
	a.mu.Lock()
	ch, ok := a.challenges[nonce]
	if !ok {
		a.mu.Unlock()
		return false, nil
	}
	if time.Since(ch.CreatedAt) > a.ttl {
		delete(a.challenges, nonce)
		a.mu.Unlock()
		return false, nil
	}

	delete(a.challenges, nonce)
	a.mu.Unlock()

	msg := "\x19Ethereum Signed Message:\n" + strconv.Itoa(len(nonce)) + nonce
	hash := crypto.Keccak256([]byte(msg))

	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return false, fmt.Errorf("auth: decode signature: %w", err)
	}
	if len(sigBytes) != 65 {
		return false, fmt.Errorf("auth: invalid signature length %d", len(sigBytes))
	}

	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	pubKey, err := crypto.SigToPub(hash, sigBytes)
	if err != nil {
		return false, fmt.Errorf("auth: recover pubkey: %w", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)

	return strings.EqualFold(recoveredAddr.Hex(), common.HexToAddress(address).Hex()), nil
}

func (a *Auth) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for nonce, ch := range a.challenges {
		if now.Sub(ch.CreatedAt) > a.ttl {
			delete(a.challenges, nonce)
		}
	}
}

func (a *Auth) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(a.ttl))
	go func() {
		for {
			select {
			case <-ticker.C:
				a.cleanup()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
