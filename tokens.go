package main

import (
	"encoding/base64"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
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

type TokenManager struct {
	CookieName string
	lock       sync.RWMutex
	tokens     map[Token]Identifier
}

func NewTokenManager(cookieName string) *TokenManager {
	return &TokenManager{
		CookieName: cookieName,
	}
}

func (m *TokenManager) ReloadFromFile(file string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	var tokens map[Token]Identifier
	err = json.Unmarshal(b, &tokens)
	if err != nil {
		return err
	}
	for token, identifier := range tokens {
		b, err := base64.StdEncoding.DecodeString(string(token))
		if err != nil {
			return err
		}
		err = bcrypt.CompareHashAndPassword(b, []byte(identifier))
		if err != nil {
			return err
		}
	}
	m.tokens = tokens
	return nil
}

func (m *TokenManager) GenerateFor(id Identifier) (Token, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(id), bcrypt.DefaultCost)
	if err != nil {
		return DefaultToken, err
	}
	return Token(base64.StdEncoding.EncodeToString(hash)), nil
}

func (m *TokenManager) Get(token Token) (Identifier, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	i, ok := m.tokens[token]
	return i, ok
}

func (m *TokenManager) GetFromRequest(r *http.Request) (Identifier, bool) {
	cookie, err := r.Cookie(m.CookieName)
	if err != nil {
		return DefaultIdentifier, false
	}
	return m.Get(Token(cookie.Value))
}
