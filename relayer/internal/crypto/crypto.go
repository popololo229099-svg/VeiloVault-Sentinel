package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/ssh"
)

type HashAlgorithm string

const (
	HashSHA256  HashAlgorithm = "sha256"
	HashSHA512  HashAlgorithm = "sha512"
	HashBlake2b HashAlgorithm = "blake2b"
)

type Hasher struct {
	algorithm HashAlgorithm
}

func NewHasher(algorithm HashAlgorithm) *Hasher {
	return &Hasher{algorithm: algorithm}
}

func (h *Hasher) Hash(data []byte) string {
	switch h.algorithm {
	case HashSHA256:
		return h.sha256(data)
	case HashSHA512:
		return h.sha512(data)
	default:
		return h.sha256(data)
	}
}

func (h *Hasher) sha256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (h *Hasher) sha512(data []byte) string {
	hash := sha512.Sum512(data)
	return hex.EncodeToString(hash[:])
}

func (h *Hasher) HashWithSalt(data []byte, salt []byte) string {
	switch h.algorithm {
	case HashSHA256:
		h := sha256.New()
		h.Write(salt)
		h.Write(data)
		return hex.EncodeToString(h.Sum(nil))
	case HashSHA512:
		h := sha512.New()
		h.Write(salt)
		h.Write(data)
		return hex.EncodeToString(h.Sum(nil))
	default:
		return h.sha256(data)
	}
}

func (h *Hasher) Verify(data []byte, hash string, salt []byte) bool {
	computed := h.HashWithSalt(data, salt)
	return computed == hash
}

type KeyDeriver struct {
	algorithm string
}

func NewKeyDeriver(algorithm string) *KeyDeriver {
	return &KeyDeriver{algorithm: algorithm}
}

func (kd *KeyDeriver) DeriveKey(password string, salt []byte, keyLen int) []byte {
	switch kd.algorithm {
	case "argon2":
		return argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, uint32(keyLen))
	case "pbkdf2":
		return pbkdf2.Key([]byte(password), salt, 100000, keyLen, sha256.New)
	case "bcrypt":
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		return hash
	default:
		return pbkdf2.Key([]byte(password), salt, 100000, keyLen, sha256.New)
	}
}

type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

type AESEncryptor struct {
	gcm cipher.AEAD
}

func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESEncryptor{gcm: gcm}, nil
}

func (e *AESEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return e.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (e *AESEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return e.gcm.Open(nil, nonce, ciphertext, nil)
}

type ChaCha20Encryptor struct {
	aead cipher.AEAD
}

func NewChaCha20Encryptor(key []byte) (*ChaCha20Encryptor, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return &ChaCha20Encryptor{aead: aead}, nil
}

func (e *ChaCha20Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return e.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (e *ChaCha20Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return e.aead.Open(nil, nonce, ciphertext, nil)
}

type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{PublicKey: pub, PrivateKey: priv}, nil
}

func (kp *KeyPair) Sign(data []byte) []byte {
	return ed25519.Sign(kp.PrivateKey, data)
}

func (kp *KeyPair) Verify(data []byte, signature []byte) bool {
	return ed25519.Verify(kp.PublicKey, data, signature)
}

func (kp *KeyPair) PublicKeyHex() string {
	return hex.EncodeToString(kp.PublicKey)
}

type HMACSigner struct {
	key []byte
}

func NewHMACSigner(key []byte) *HMACSigner {
	return &HMACSigner{key: key}
}

func (s *HMACSigner) Sign(data []byte) string {
	h := hmac.New(sha256.New, s.key)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *HMACSigner) Verify(data []byte, signature string) bool {
	return s.Sign(data) == signature
}

func GenerateRandomBytes(n int) ([]byte, error) {
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	return bytes, err
}

func GenerateRandomHex(n int) (string, error) {
	bytes, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateRandomString(length int, charset string) string {
	if charset == "" {
		charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}
	result := make([]byte, length)
	for i := range result {
		b, _ := GenerateRandomBytes(1)
		result[i] = charset[int(b[0])%len(charset)]
	}
	return string(result)
}

type PasswordHasher struct {
	algorithm string
	cost     int
}

func NewPasswordHasher(algorithm string) *PasswordHasher {
	return &PasswordHasher{algorithm: algorithm}
}

func (ph *PasswordHasher) Hash(password string) (string, error) {
	switch ph.algorithm {
	case "bcrypt":
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		return string(hash), err
	case "argon2":
		salt := make([]byte, 16)
		rand.Read(salt)
		hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
		return fmt.Sprintf("%s:%s:%s", hex.EncodeToString(salt), hex.EncodeToString(hash), "argon2"), nil
	case "pbkdf2":
		salt := make([]byte, 16)
		rand.Read(salt)
		hash := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
		return fmt.Sprintf("%s:%s:%s", hex.EncodeToString(salt), hex.EncodeToString(hash), "pbkdf2"), nil
	default:
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		return string(hash), err
	}
}

func (ph *PasswordHasher) Verify(password, hashStr string) bool {
	parts := strings.SplitN(hashStr, ":", 3)
	if len(parts) == 3 {
		salt, _ := hex.DecodeString(parts[0])
		expectedHash, _ := hex.DecodeString(parts[1])
		algorithm := parts[2]
		switch algorithm {
		case "argon2":
			computed := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, uint32(len(expectedHash)))
			return string(computed) == string(expectedHash)
		case "pbkdf2":
			computed := pbkdf2.Key([]byte(password), salt, 100000, len(expectedHash), sha256.New)
			return string(computed) == string(expectedHash)
		}
	}
	return bcrypt.CompareHashAndPassword([]byte(hashStr), []byte(password)) == nil
}

type SecureToken struct {
	Value     string
	ExpiresAt time.Time
}

func GenerateSecureToken(ttl time.Duration) *SecureToken {
	token := make([]byte, 32)
	rand.Read(token)
	return &SecureToken{
		Value:     hex.EncodeToString(token),
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (t *SecureToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

func (t *SecureToken) RemainingTime() time.Duration {
	remaining := time.Until(t.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

type KeyManager struct {
	keys    map[string][]byte
	keyTTL  map[string]time.Time
	mu      sync.RWMutex
}

func NewKeyManager() *KeyManager {
	return &KeyManager{
		keys:   make(map[string][]byte),
		keyTTL: make(map[string]time.Time),
	}
}

func (km *KeyManager) StoreKey(id string, key []byte, ttl time.Duration) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.keys[id] = key
	if ttl > 0 {
		km.keyTTL[id] = time.Now().Add(ttl)
	}
}

func (km *KeyManager) GetKey(id string) ([]byte, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	key, ok := km.keys[id]
	if !ok {
		return nil, false
	}
	if expiry, exists := km.keyTTL[id]; exists && time.Now().After(expiry) {
		return nil, false
	}
	return key, true
}

func (km *KeyManager) RotateKey(id string, keyLen int) ([]byte, error) {
	newKey, err := GenerateRandomBytes(keyLen)
	if err != nil {
		return nil, err
	}
	km.mu.Lock()
	defer km.mu.Unlock()
	km.keys[id] = newKey
	return newKey, nil
}

func (km *KeyManager) DeleteKey(id string) {
	km.mu.Lock()
	defer km.mu.Unlock()
	delete(km.keys, id)
	delete(km.keyTTL, id)
}

func (km *KeyManager) ListKeys() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()
	keys := make([]string, 0, len(km.keys))
	for k := range km.keys {
		keys = append(keys, k)
	}
	return keys
}

func (km *KeyManager) Cleanup() {
	km.mu.Lock()
	defer km.mu.Unlock()
	now := time.Now()
	for id, expiry := range km.keyTTL {
		if now.After(expiry) {
			delete(km.keys, id)
			delete(km.keyTTL, id)
		}
	}
}

type SSHKeyManager struct {
	signers map[string]ssh.Signer
	mu      sync.RWMutex
}

func NewSSHKeyManager() *SSHKeyManager {
	return &SSHKeyManager{signers: make(map[string]ssh.Signer)}
}

func (m *SSHKeyManager) AddKey(id string, pemData []byte, passphrase []byte) error {
	var signer ssh.Signer
	var err error
	if len(passphrase) > 0 {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(pemData, passphrase)
	} else {
		signer, err = ssh.ParsePrivateKey(pemData)
	}
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signers[id] = signer
	return nil
}

func (m *SSHKeyManager) GetSigner(id string) (ssh.Signer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.signers[id]
	return s, ok
}

func (m *SSHKeyManager) RemoveKey(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.signers, id)
}

type Vault struct {
	data   map[string][]byte
	gcm    cipher.AEAD
	mu     sync.RWMutex
}

func NewVault(encryptionKey []byte) (*Vault, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{
		data: make(map[string][]byte),
		gcm:  gcm,
	}, nil
}

func (v *Vault) Store(key string, plaintext []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	encrypted := v.gcm.Seal(nonce, nonce, plaintext, nil)
	v.data[key] = encrypted
	return nil
}

func (v *Vault) Retrieve(key string) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	encrypted, ok := v.data[key]
	if !ok {
		return nil, fmt.Errorf("key %s not found", key)
	}
	nonceSize := v.gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("invalid data")
	}
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	return v.gcm.Open(nil, nonce, ciphertext, nil)
}

func (v *Vault) Delete(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.data, key)
}

func (v *Vault) Exists(key string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	_, ok := v.data[key]
	return ok
}

func (v *Vault) List() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	keys := make([]string, 0, len(v.data))
	for k := range v.data {
		keys = append(keys, k)
	}
	return keys
}

func XORBytes(a, b []byte) []byte {
	result := make([]byte, len(a))
	for i := range a {
		if i < len(b) {
			result[i] = a[i] ^ b[i]
		} else {
			result[i] = a[i]
		}
	}
	return result
}

func ConstantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	result := byte(0)
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func SecureWipe(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
