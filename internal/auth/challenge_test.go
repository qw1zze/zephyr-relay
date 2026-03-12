package auth_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"log/slog"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"zephyr-relay/internal/auth"
)

func newTestAuth(ttl time.Duration) *auth.Auth {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return auth.NewWithTTL(context.Background(), ttl, log)
}

func signMessage(t *testing.T, privateKey *ecdsa.PrivateKey, message string) string {
	t.Helper()
	msg := "\x19Ethereum Signed Message:\n" +
		strconv.Itoa(len(message)) + message
	hash := crypto.Keccak256([]byte(msg))
	sig, err := crypto.Sign(hash, privateKey)
	require.NoError(t, err)
	sig[64] += 27
	return "0x" + hex.EncodeToString(sig)
}

func TestNewChallenge_Success(t *testing.T) {
	a := newTestAuth(60 * time.Second)
	nonce, err := a.NewChallenge()
	require.NoError(t, err)
	require.NotEmpty(t, nonce)
	require.Len(t, nonce, 64)
	_, err = hex.DecodeString(nonce)
	require.NoError(t, err)
}

func TestNewChallenge_Unique(t *testing.T) {
	a := newTestAuth(60 * time.Second)
	n1, err := a.NewChallenge()
	require.NoError(t, err)
	n2, err := a.NewChallenge()
	require.NoError(t, err)
	require.NotEqual(t, n1, n2)
}

func TestVerify_Success(t *testing.T) {
	a := newTestAuth(60 * time.Second)

	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	nonce, err := a.NewChallenge()
	require.NoError(t, err)

	sig := signMessage(t, privKey, nonce)
	addr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	ok, err := a.Verify(nonce, addr, sig)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestVerify_WrongAddress(t *testing.T) {
	a := newTestAuth(60 * time.Second)

	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	wrongKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	nonce, err := a.NewChallenge()
	require.NoError(t, err)

	sig := signMessage(t, privKey, nonce)
	wrongAddr := crypto.PubkeyToAddress(wrongKey.PublicKey).Hex()

	ok, err := a.Verify(nonce, wrongAddr, sig)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerify_InvalidSignature(t *testing.T) {
	a := newTestAuth(60 * time.Second)

	nonce, err := a.NewChallenge()
	require.NoError(t, err)

	ok, err := a.Verify(nonce, "0x1234567890123456789012345678901234567890", "0xinvalid")
	require.Error(t, err)
	require.False(t, ok)
}

func TestVerify_UnknownNonce(t *testing.T) {
	a := newTestAuth(60 * time.Second)

	ok, err := a.Verify("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"0x1234567890123456789012345678901234567890", "0xsig")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerify_ExpiredNonce(t *testing.T) {
	a := newTestAuth(1 * time.Millisecond)

	nonce, err := a.NewChallenge()
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)

	ok, err := a.Verify(nonce, "0x1234567890123456789012345678901234567890", "0xsig")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerify_ReplayAttack(t *testing.T) {
	a := newTestAuth(60 * time.Second)

	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	nonce, err := a.NewChallenge()
	require.NoError(t, err)

	sig := signMessage(t, privKey, nonce)
	addr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	ok, err := a.Verify(nonce, addr, sig)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = a.Verify(nonce, addr, sig)
	require.NoError(t, err)
	require.False(t, ok)
}
