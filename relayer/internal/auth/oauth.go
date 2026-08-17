package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OAuth2Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	Scopes       []string
	AccessType   string
}

type OAuth2Token struct {
	AccessToken  string
	TokenType    string
	RefreshToken string
	ExpiresIn    int64
	Scope        string
	CreatedAt    time.Time
	mu           sync.RWMutex
}

func (t *OAuth2Token) IsValid() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.ExpiresIn == 0 {
		return true
	}
	expiry := t.CreatedAt.Add(time.Duration(t.ExpiresIn) * time.Second)
	return time.Now().Before(expiry)
}

func (t *OAuth2Token) RemainingTime() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.ExpiresIn == 0 {
		return time.Duration(1<<63 - 1)
	}
	expiry := t.CreatedAt.Add(time.Duration(t.ExpiresIn) * time.Second)
	remaining := time.Until(expiry)
	if remaining < 0 {
		return 0
	}
	return remaining
}

type OAuth2Flow struct {
	config     OAuth2Config
	stateStore map[string]time.Time
	mu         sync.RWMutex
	httpClient HTTPClient
}

type HTTPClient interface {
	Get(url string) ([]byte, error)
	Post(url string, data url.Values) ([]byte, error)
}

type DefaultHTTPClient struct{}

func (d *DefaultHTTPClient) Get(url string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *DefaultHTTPClient) Post(url string, data url.Values) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func NewOAuth2Flow(config OAuth2Config) *OAuth2Flow {
	return &OAuth2Flow{
		config:     config,
		stateStore: make(map[string]time.Time),
		httpClient: &DefaultHTTPClient{},
	}
}

func (of *OAuth2Flow) SetHTTPClient(client HTTPClient) {
	of.mu.Lock()
	defer of.mu.Unlock()
	of.httpClient = client
}

func (of *OAuth2Flow) GetAuthorizationURL(state string) string {
	if state == "" {
		state = generateOAuthState()
	}

	params := url.Values{
		"client_id":     {of.config.ClientID},
		"redirect_uri":  {of.config.RedirectURL},
		"response_type": {"code"},
		"state":         {state},
	}

	if len(of.config.Scopes) > 0 {
		params.Set("scope", strings.Join(of.config.Scopes, " "))
	}

	if of.config.AccessType != "" {
		params.Set("access_type", of.config.AccessType)
	}

	of.mu.Lock()
	of.stateStore[state] = time.Now()
	of.mu.Unlock()

	return of.config.AuthURL + "?" + params.Encode()
}

func (of *OAuth2Flow) ValidateState(state string) bool {
	of.mu.Lock()
	defer of.mu.Unlock()

	storedTime, exists := of.stateStore[state]
	if !exists {
		return false
	}

	delete(of.stateStore, state)

	if time.Since(storedTime) > 10*time.Minute {
		return false
	}

	return true
}

func (of *OAuth2Flow) ExchangeCode(code string) (*OAuth2Token, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {of.config.RedirectURL},
		"client_id":     {of.config.ClientID},
		"client_secret": {of.config.ClientSecret},
	}

	resp, err := of.httpClient.Post(of.config.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	token := &OAuth2Token{
		CreatedAt: time.Now(),
	}

	if err := parseTokenResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

func (of *OAuth2Flow) RefreshToken(refreshToken string) (*OAuth2Token, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {of.config.ClientID},
		"client_secret": {of.config.ClientSecret},
	}

	resp, err := of.httpClient.Post(of.config.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	token := &OAuth2Token{
		CreatedAt: time.Now(),
	}

	if err := parseTokenResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

func (of *OAuth2Flow) CleanupStates() {
	of.mu.Lock()
	defer of.mu.Unlock()

	for state, created := range of.stateStore {
		if time.Since(created) > 10*time.Minute {
			delete(of.stateStore, state)
		}
	}
}

func (of *OAuth2Flow) StateCount() int {
	of.mu.RLock()
	defer of.mu.RUnlock()
	return len(of.stateStore)
}

type APIKey struct {
	ID        string
	Key       string
	Secret    string
	Name      string
	Scopes    []string
	RateLimit int
	ExpiresAt *time.Time
	CreatedAt time.Time
	LastUsed  *time.Time
	Revoked   bool
	Metadata  map[string]string
	mu        sync.RWMutex
}

func (ak *APIKey) IsValid() bool {
	ak.mu.RLock()
	defer ak.mu.RUnlock()
	if ak.Revoked {
		return false
	}
	if ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt) {
		return false
	}
	return true
}

func (ak *APIKey) HasScope(scope string) bool {
	ak.mu.RLock()
	defer ak.mu.RUnlock()
	for _, s := range ak.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

func (ak *APIKey) MarkUsed() {
	ak.mu.Lock()
	defer ak.mu.Unlock()
	now := time.Now()
	ak.LastUsed = &now
}

type APIKeyManager struct {
	keys map[string]*APIKey
	mu   sync.RWMutex
}

func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys: make(map[string]*APIKey),
	}
}

func (akm *APIKeyManager) GenerateKey(name string, scopes []string, rateLimit int, expiry *time.Time) (*APIKey, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}

	key := &APIKey{
		ID:        generateKeyID(),
		Key:       base64.URLEncoding.EncodeToString(keyBytes),
		Secret:    base64.URLEncoding.EncodeToString(secretBytes),
		Name:      name,
		Scopes:    scopes,
		RateLimit: rateLimit,
		ExpiresAt: expiry,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}

	akm.mu.Lock()
	defer akm.mu.Unlock()
	akm.keys[key.ID] = key
	return key, nil
}

func (akm *APIKeyManager) ValidateKey(keyValue string) (*APIKey, bool) {
	akm.mu.RLock()
	defer akm.mu.RUnlock()

	for _, key := range akm.keys {
		if key.Key == keyValue && key.IsValid() {
			key.MarkUsed()
			return key, true
		}
	}
	return nil, false
}

func (akm *APIKeyManager) ValidateKeySecret(keyValue, secretValue string) (*APIKey, bool) {
	akm.mu.RLock()
	defer akm.mu.RUnlock()

	for _, key := range akm.keys {
		if key.Key == keyValue && key.Secret == secretValue && key.IsValid() {
			key.MarkUsed()
			return key, true
		}
	}
	return nil, false
}

func (akm *APIKeyManager) RevokeKey(id string) bool {
	akm.mu.Lock()
	defer akm.mu.Unlock()

	key, exists := akm.keys[id]
	if !exists {
		return false
	}
	key.mu.Lock()
	defer key.mu.Unlock()
	key.Revoked = true
	return true
}

func (akm *APIKeyManager) DeleteKey(id string) bool {
	akm.mu.Lock()
	defer akm.mu.Unlock()
	_, exists := akm.keys[id]
	if exists {
		delete(akm.keys, id)
		return true
	}
	return false
}

func (akm *APIKeyManager) GetKey(id string) *APIKey {
	akm.mu.RLock()
	defer akm.mu.RUnlock()
	return akm.keys[id]
}

func (akm *APIKeyManager) ListKeys() []*APIKey {
	akm.mu.RLock()
	defer akm.mu.RUnlock()

	keys := make([]*APIKey, 0, len(akm.keys))
	for _, key := range akm.keys {
		keys = append(keys, key)
	}
	return keys
}

func (akm *APIKeyManager) ActiveKeyCount() int {
	akm.mu.RLock()
	defer akm.mu.RUnlock()

	count := 0
	for _, key := range akm.keys {
		if key.IsValid() {
			count++
		}
	}
	return count
}

func (akm *APIKeyManager) Cleanup() int {
	akm.mu.Lock()
	defer akm.mu.Unlock()

	count := 0
	for id, key := range akm.keys {
		if key.Revoked || (key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt)) {
			delete(akm.keys, id)
			count++
		}
	}
	return count
}

type APIKeyMiddlewareConfig struct {
	HeaderName     string
	Prefix         string
	Validator      func(string) (*APIKey, bool)
	OnUnauthorized func(w interface{}, r interface{})
}

func generateOAuthState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func generateKeyID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ak_" + base64.URLEncoding.EncodeToString(b)
}

func parseTokenResponse(data []byte, token *OAuth2Token) error {
	token.AccessToken = string(data)
	return nil
}

type contextKey string

const (
	APIKeyContextKey   contextKey = "api_key"
	UserContextKey     contextKey = "user"
	ClaimsContextKey   contextKey = "claims"
	PermissionsContextKey contextKey = "permissions"
)

func ContextWithAPIKey(ctx context.Context, key *APIKey) context.Context {
	return context.WithValue(ctx, APIKeyContextKey, key)
}

func APIKeyFromContext(ctx context.Context) (*APIKey, bool) {
	key, ok := ctx.Value(APIKeyContextKey).(*APIKey)
	return key, ok
}

func ContextWithClaims(ctx context.Context, claims *JWTClaims) context.Context {
	return context.WithValue(ctx, ClaimsContextKey, claims)
}

func ClaimsFromContext(ctx context.Context) (*JWTClaims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*JWTClaims)
	return claims, ok
}

func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserContextKey).(*User)
	return user, ok
}
