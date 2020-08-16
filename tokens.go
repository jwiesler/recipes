package main

import (
	hmac2 "crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io/ioutil"
	"net/http"
	"sync"
)

type Token string
type Identifier string

const (
	DefaultIdentifier Identifier = ""
	DefaultToken      Token      = ""
)

type HMacTokenSource struct {
	hash hash.Hash
	lock sync.Mutex
}

func NewHMacTokenSource(key []byte) *HMacTokenSource {
	return &HMacTokenSource{
		hash: hmac2.New(sha256.New, key),
	}
}

var ErrDoesNotMatch = errors.New("hmac does not match")
var ErrInvalidKeySize = errors.New("invalid key size")

func (m *HMacTokenSource) VerifyIsGeneratedBy(token []byte, by []byte) error {
	b, err := m.Generate(by)
	if err != nil {
		return err
	}
	if m.Verify(token, b) {
		return nil
	}
	return ErrDoesNotMatch
}

func (m *HMacTokenSource) Verify(token []byte, actual []byte) bool {
	return hmac2.Equal(token, actual)
}

func (m *HMacTokenSource) Generate(value []byte) ([]byte, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.hash.Reset()
	_, err := m.hash.Write(value)
	if err != nil {
		return nil, err
	}
	return m.hash.Sum(nil), nil
}

type TokenManager struct {
	source     *HMacTokenSource
	CookieName string
	lock       sync.RWMutex
	tokens     map[Token]Identifier
}

func NewTokenManager(cookieName string, key []byte) *TokenManager {
	return &TokenManager{
		CookieName: cookieName,
		source: NewHMacTokenSource(key),
	}
}

func DecodeToken(token Token) ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(token))
}

func (m *TokenManager) ReloadFromFile(file string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	var tokens map[Identifier]Token
	err = json.Unmarshal(b, &tokens)
	if err != nil {
		return err
	}
	res := make(map[Token]Identifier, len(tokens))
	for identifier, token := range tokens {
		b, err := DecodeToken(token)
		if err != nil {
			return err
		}
		err = m.source.VerifyIsGeneratedBy(b, []byte(identifier))
		if err != nil {
			return err
		}
		res[token] = identifier
	}
	m.tokens = res
	return nil
}

func (m *TokenManager) Get(token Token) (Identifier, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	i, ok := m.tokens[token]
	return i, ok
}

func (m *TokenManager) GetTokenFromRequest(r *http.Request) (Token, bool) {
	cookie, err := r.Cookie(m.CookieName)
	if err != nil {
		return DefaultToken, false
	}
	return Token(cookie.Value), true
}

func (m *TokenManager) GetFromRequest(r *http.Request) (Identifier, bool) {
	token, ok := m.GetTokenFromRequest(r)
	if !ok {
		return DefaultIdentifier, false
	}
	return m.Get(token)
}

func ReadTokensKeyFile(path string) ([]byte, error) {
	hexBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(string(hexBytes))
}
