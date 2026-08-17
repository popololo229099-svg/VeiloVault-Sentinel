package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"sync"
	"time"
)

type JWTClaims struct {
	Issuer    string                 `json:"iss"`
	Subject   string                 `json:"sub"`
	Audience  string                 `json:"aud"`
	Expiry    int64                  `json:"exp"`
	NotBefore int64                  `json:"nbf"`
	IssuedAt  int64                  `json:"iat"`
	ID        string                 `json:"jti"`
	Custom    map[string]interface{} `json:"custom,omitempty"`
}

type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

type JWTToken struct {
	Raw       string
	Header    JWTHeader
	Claims    JWTClaims
	Signature []byte
	Valid     bool
}

type JWTConfig struct {
	Secret        string
	SigningMethod  string
	Expiry        time.Duration
	Issuer        string
	AllowRefresh  bool
	RefreshWindow time.Duration
	ClockSkew     time.Duration
}

func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SigningMethod:  "HS256",
		Expiry:         time.Hour,
		RefreshWindow:  time.Minute * 5,
		ClockSkew:      time.Second * 30,
	}
}

type JWTManager struct {
	config   JWTConfig
	keyFunc  func(kid string) (string, error)
	blacklist map[string]bool
	mu       sync.RWMutex
}

func NewJWTManager(config JWTConfig) *JWTManager {
	return &JWTManager{
		config:    config,
		blacklist: make(map[string]bool),
	}
}

func (jm *JWTManager) SetKeyFunc(fn func(kid string) (string, error)) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.keyFunc = fn
}

func (jm *JWTManager) CreateToken(claims JWTClaims) (string, error) {
	if claims.Expiry == 0 {
		claims.Expiry = time.Now().Add(jm.config.Expiry).Unix()
	}
	if claims.IssuedAt == 0 {
		claims.IssuedAt = time.Now().Unix()
	}
	if claims.Issuer == "" {
		claims.Issuer = jm.config.Issuer
	}
	if claims.ID == "" {
		claims.ID = generateJTI()
	}

	header := JWTHeader{
		Alg: jm.config.SigningMethod,
		Typ: "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}

	headerB64 := base64URLEncode(headerJSON)
	claimsB64 := base64URLEncode(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	signature, err := jm.sign(signingInput)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	token := signingInput + "." + base64URLEncode(signature)
	return token, nil
}

func (jm *JWTManager) ValidateToken(tokenString string) (*JWTToken, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	var header JWTHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("failed to parse header: %w", err)
	}

	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig, err := jm.sign(signingInput)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	receivedSig, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	if !hmac.Equal(expectedSig, receivedSig) {
		return &JWTToken{
			Raw:    tokenString,
			Header: header,
			Claims: claims,
			Valid:  false,
		}, fmt.Errorf("invalid signature")
	}

	jm.mu.RLock()
	blacklisted := jm.blacklist[claims.ID]
	jm.mu.RUnlock()

	if blacklisted {
		return &JWTToken{
			Raw:    tokenString,
			Header: header,
			Claims: claims,
			Valid:  false,
		}, fmt.Errorf("token is blacklisted")
	}

	now := time.Now()
	skew := jm.config.ClockSkew

	if claims.Expiry > 0 && now.Unix() > claims.Expiry+int64(skew.Seconds()) {
		return &JWTToken{
			Raw:    tokenString,
			Header: header,
			Claims: claims,
			Valid:  false,
		}, fmt.Errorf("token has expired")
	}

	if claims.NotBefore > 0 && now.Unix() < claims.NotBefore-int64(skew.Seconds()) {
		return &JWTToken{
			Raw:    tokenString,
			Header: header,
			Claims: claims,
			Valid:  false,
		}, fmt.Errorf("token is not yet valid")
	}

	return &JWTToken{
		Raw:    tokenString,
		Header: header,
		Claims: claims,
		Signature: receivedSig,
		Valid:  true,
	}, nil
}

func (jm *JWTManager) RefreshToken(tokenString string) (string, error) {
	if !jm.config.AllowRefresh {
		return "", fmt.Errorf("refresh not allowed")
	}

	token, err := jm.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	if !token.Valid {
		return "", fmt.Errorf("token is not valid")
	}

	now := time.Now()
	refreshDeadline := time.Unix(token.Claims.Expiry, 0).Add(-jm.config.RefreshWindow)

	if now.After(refreshDeadline) {
		return "", fmt.Errorf("token is outside refresh window")
	}

	newClaims := JWTClaims{
		Issuer:    token.Claims.Issuer,
		Subject:   token.Claims.Subject,
		Audience:  token.Claims.Audience,
		Custom:    token.Claims.Custom,
	}

	return jm.CreateToken(newClaims)
}

func (jm *JWTManager) BlacklistToken(tokenID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.blacklist[tokenID] = true
}

func (jm *JWTManager) IsBlacklisted(tokenID string) bool {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.blacklist[tokenID]
}

func (jm *JWTManager) ExtractClaims(tokenString string) (*JWTClaims, error) {
	token, err := jm.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	return &token.Claims, nil
}

func (jm *JWTManager) sign(input string) ([]byte, error) {
	secret := jm.config.Secret

	if jm.keyFunc != nil {
		s, err := jm.keyFunc("")
		if err != nil {
			return nil, err
		}
		secret = s
	}

	switch jm.config.SigningMethod {
	case "HS256":
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(input))
		return mac.Sum(nil), nil
	case "HS384":
		mac := hmac.New(sha512.New384, []byte(secret))
		mac.Write([]byte(input))
		return mac.Sum(nil), nil
	case "HS512":
		mac := hmac.New(sha512.New, []byte(secret))
		mac.Write([]byte(input))
		return mac.Sum(nil), nil
	default:
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(input))
		return mac.Sum(nil), nil
	}
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func base64URLDecode(s string) ([]byte, error) {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

func generateJTI() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64URLEncode(b)
}

type HMACSigner struct {
	hashFunc func() hash.Hash
	key      []byte
	mu       sync.RWMutex
}

func NewHMACSigner(key string) *HMACSigner {
	return &HMACSigner{
		hashFunc: sha256.New,
		key:      []byte(key),
	}
}

func (hs *HMACSigner) Sign(data []byte) ([]byte, error) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	mac := hmac.New(sha256.New, hs.key)
	mac.Write(data)
	return mac.Sum(nil), nil
}

func (hs *HMACSigner) Verify(data, signature []byte) bool {
	expected, err := hs.Sign(data)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, signature)
}

func (hs *HMACSigner) SetKey(key string) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.key = []byte(key)
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type TokenManager struct {
	jwtManager *JWTManager
	config     TokenConfig
	mu         sync.RWMutex
}

type TokenConfig struct {
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	Issuer             string
}

func DefaultTokenConfig() TokenConfig {
	return TokenConfig{
		AccessTokenExpiry:  time.Hour,
		RefreshTokenExpiry: 24 * time.Hour * 7,
	}
}

func NewTokenManager(config TokenConfig) *TokenManager {
	jwtConfig := JWTConfig{
		Secret:       "default-secret-change-me",
		SigningMethod: "HS256",
		Expiry:       config.AccessTokenExpiry,
		Issuer:       config.Issuer,
	}

	return &TokenManager{
		jwtManager: NewJWTManager(jwtConfig),
		config:     config,
	}
}

func (tm *TokenManager) GenerateTokenPair(subject string, customClaims map[string]interface{}) (*TokenPair, error) {
	accessClaims := JWTClaims{
		Subject:  subject,
		Audience: "access",
		Custom:   customClaims,
	}

	accessToken, err := tm.jwtManager.CreateToken(accessClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to create access token: %w", err)
	}

	refreshClaims := JWTClaims{
		Subject:  subject,
		Audience: "refresh",
		Expiry:   time.Now().Add(tm.config.RefreshTokenExpiry).Unix(),
	}

	refreshToken, err := tm.jwtManager.CreateToken(refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(tm.config.AccessTokenExpiry),
	}, nil
}

func (tm *TokenManager) ValidateAccessToken(token string) (*JWTClaims, error) {
	claims, err := tm.jwtManager.ExtractClaims(token)
	if err != nil {
		return nil, err
	}

	if claims.Audience != "access" {
		return nil, fmt.Errorf("invalid token audience")
	}

	return claims, nil
}

func (tm *TokenManager) RefreshAccessToken(refreshToken string) (string, error) {
	claims, err := tm.jwtManager.ExtractClaims(refreshToken)
	if err != nil {
		return "", err
	}

	if claims.Audience != "refresh" {
		return "", fmt.Errorf("invalid refresh token audience")
	}

	newAccessClaims := JWTClaims{
		Subject:  claims.Subject,
		Audience: "access",
		Custom:   claims.Custom,
	}

	return tm.jwtManager.CreateToken(newAccessClaims)
}

func (tm *TokenManager) SetSecret(secret string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.jwtManager.config.Secret = secret
}

type SignedRequest struct {
	Timestamp string
	Nonce     string
	Signature string
}

type RequestSigner struct {
	secret    string
	tolerance time.Duration
	mu        sync.RWMutex
}

func NewRequestSigner(secret string) *RequestSigner {
	return &RequestSigner{
		secret:    secret,
		tolerance: 5 * time.Minute,
	}
}

func (rs *RequestSigner) Sign(method, path, body string) (*SignedRequest, error) {
	rs.mu.RLock()
	secret := rs.secret
	rs.mu.RUnlock()

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := generateNonce()

	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, body, timestamp, nonce)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	signature := base64URLEncode(mac.Sum(nil))

	return &SignedRequest{
		Timestamp: timestamp,
		Nonce:     nonce,
		Signature: signature,
	}, nil
}

func (rs *RequestSigner) Verify(req *SignedRequest, method, path, body string) bool {
	rs.mu.RLock()
	secret := rs.secret
	tolerance := rs.tolerance
	rs.mu.RUnlock()

	ts, err := parseTimestamp(req.Timestamp)
	if err != nil {
		return false
	}

	if time.Since(ts) > tolerance {
		return false
	}

	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, body, req.Timestamp, req.Nonce)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expected := base64URLEncode(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(req.Signature))
}

func (rs *RequestSigner) SetTolerance(tolerance time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.tolerance = tolerance
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64URLEncode(b)
}

func parseTimestamp(s string) (time.Time, error) {
	var ts int64
	_, err := fmt.Sscanf(s, "%d", &ts)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

type SignerPool struct {
	signers map[string]*RequestSigner
	mu      sync.RWMutex
}

func NewSignerPool() *SignerPool {
	return &SignerPool{
		signers: make(map[string]*RequestSigner),
	}
}

func (sp *SignerPool) AddSigner(name, secret string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.signers[name] = NewRequestSigner(secret)
}

func (sp *SignerPool) GetSigner(name string) *RequestSigner {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.signers[name]
}

func (sp *SignerPool) RemoveSigner(name string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	delete(sp.signers, name)
}

func (sp *SignerPool) SignerCount() int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return len(sp.signers)
}
