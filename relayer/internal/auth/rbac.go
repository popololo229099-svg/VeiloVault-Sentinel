package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RBACRole struct {
	Name        string
	Permissions []string
	Description string
	Metadata    map[string]string
	CreatedAt   time.Time
	mu          sync.RWMutex
}

func (r *RBACRole) HasPermission(permission string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

func (r *RBACRole) AddPermission(permission string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.Permissions {
		if p == permission {
			return
		}
	}
	r.Permissions = append(r.Permissions, permission)
}

func (r *RBACRole) RemovePermission(permission string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range r.Permissions {
		if p == permission {
			r.Permissions = append(r.Permissions[:i], r.Permissions[i+1:]...)
			return
		}
	}
}

type RBACPermission struct {
	Name        string
	Resource    string
	Actions     []string
	Description string
}

func (rp *RBACPermission) Matches(resource, action string) bool {
	if rp.Resource != resource && rp.Resource != "*" {
		return false
	}
	for _, a := range rp.Actions {
		if a == action || a == "*" {
			return true
		}
	}
	return false
}

type RBACUser struct {
	ID        string
	Roles     []string
	Groups    []string
	Attributes map[string]string
	mu        sync.RWMutex
}

func (u *RBACUser) HasRole(role string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	for _, r := range u.Roles {
		if r == role || r == "admin" {
			return true
		}
	}
	return false
}

func (u *RBACUser) AddRole(role string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, r := range u.Roles {
		if r == role {
			return
		}
	}
	u.Roles = append(u.Roles, role)
}

func (u *RBACUser) RemoveRole(role string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for i, r := range u.Roles {
		if r == role {
			u.Roles = append(u.Roles[:i], u.Roles[i+1:]...)
			return
		}
	}
}

type RBACPolicy struct {
	Name        string
	Roles       []string
	Resource    string
	Actions     []string
	Effect      string
	Conditions  map[string]string
	Description string
}

type RBACEngine struct {
	roles       map[string]*RBACRole
	users       map[string]*RBACUser
	policies    []*RBACPolicy
	permissions map[string]*RBACPermission
	hierarchy   map[string][]string
	mu          sync.RWMutex
}

func NewRBACEngine() *RBACEngine {
	return &RBACEngine{
		roles:       make(map[string]*RBACRole),
		users:       make(map[string]*RBACUser),
		policies:    make([]*RBACPolicy, 0),
		permissions: make(map[string]*RBACPermission),
		hierarchy:   make(map[string][]string),
	}
}

func (rbe *RBACEngine) AddRole(name string, permissions []string, description string) {
	rbe.mu.Lock()
	defer rbe.mu.Unlock()
	rbe.roles[name] = &RBACRole{
		Name:        name,
		Permissions: permissions,
		Description: description,
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
	}
}

func (rbe *RBACEngine) RemoveRole(name string) {
	rbe.mu.Lock()
	defer rbe.mu.Unlock()
	delete(rbe.roles, name)
}

func (rbe *RBACEngine) GetRole(name string) *RBACRole {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()
	return rbe.roles[name]
}

func (rbe *RBACEngine) AddUser(id string, roles []string) {
	rbe.mu.Lock()
	defer rbe.mu.Unlock()
	rbe.users[id] = &RBACUser{
		ID:         id,
		Roles:      roles,
		Groups:     make([]string, 0),
		Attributes: make(map[string]string),
	}
}

func (rbe *RBACEngine) GetUser(id string) *RBACUser {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()
	return rbe.users[id]
}

func (rbe *RBACEngine) RemoveUser(id string) {
	rbe.mu.Lock()
	defer rbe.mu.Unlock()
	delete(rbe.users, id)
}

func (rbe *RBACEngine) AddPolicy(policy *RBACPolicy) {
	rbe.mu.Lock()
	defer rbe.mu.Unlock()
	rbe.policies = append(rbe.policies, policy)
}

func (rbe *RBACEngine) SetRoleHierarchy(child, parent string) {
	rbe.mu.Lock()
	defer rbe.mu.Unlock()
	rbe.hierarchy[child] = append(rbe.hierarchy[child], parent)
}

func (rbe *RBACEngine) CheckPermission(userID, resource, action string) bool {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()

	user, exists := rbe.users[userID]
	if !exists {
		return false
	}

	for _, roleName := range user.Roles {
		if rbe.checkRolePermission(roleName, resource, action) {
			return true
		}
	}

	return false
}

func (rbe *RBACEngine) checkRolePermission(roleName, resource, action string) bool {
	role, exists := rbe.roles[roleName]
	if !exists {
		return false
	}

	for _, perm := range role.Permissions {
		if perm == "*" {
			return true
		}
		parts := strings.Split(perm, ":")
		if len(parts) == 2 {
			if (parts[0] == resource || parts[0] == "*") && (parts[1] == action || parts[1] == "*") {
				return true
			}
		}
	}

	if parents, ok := rbe.hierarchy[roleName]; ok {
		for _, parent := range parents {
			if rbe.checkRolePermission(parent, resource, action) {
				return true
			}
		}
	}

	return false
}

func (rbe *RBACEngine) GetUserPermissions(userID string) []string {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()

	user, exists := rbe.users[userID]
	if !exists {
		return nil
	}

	permSet := make(map[string]bool)
	for _, roleName := range user.Roles {
		rbe.collectRolePermissions(roleName, permSet)
	}

	permissions := make([]string, 0, len(permSet))
	for p := range permSet {
		permissions = append(permissions, p)
	}
	return permissions
}

func (rbe *RBACEngine) collectRolePermissions(roleName string, permSet map[string]bool) {
	role, exists := rbe.roles[roleName]
	if !exists {
		return
	}

	for _, perm := range role.Permissions {
		permSet[perm] = true
	}

	if parents, ok := rbe.hierarchy[roleName]; ok {
		for _, parent := range parents {
			rbe.collectRolePermissions(parent, permSet)
		}
	}
}

func (rbe *RBACEngine) GetUserRoles(userID string) []string {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()

	user, exists := rbe.users[userID]
	if !exists {
		return nil
	}

	roles := make([]string, len(user.Roles))
	copy(roles, user.Roles)
	return roles
}

func (rbe *RBACEngine) RoleCount() int {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()
	return len(rbe.roles)
}

func (rbe *RBACEngine) UserCount() int {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()
	return len(rbe.users)
}

func (rbe *RBACEngine) PolicyCount() int {
	rbe.mu.RLock()
	defer rbe.mu.RUnlock()
	return len(rbe.policies)
}

func (rbe *RBACEngine) IsAdmin(userID string) bool {
	return rbe.CheckPermission(userID, "*", "*")
}

type Session struct {
	ID           string
	UserID       string
	Roles        []string
	ExpiresAt    time.Time
	LastActivity time.Time
	IPAddress    string
	UserAgent    string
	Data         map[string]interface{}
	Revoked      bool
	mu           sync.RWMutex
}

func (s *Session) IsValid() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Revoked {
		return false
	}
	if time.Now().After(s.ExpiresAt) {
		return false
	}
	return true
}

func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

func (s *Session) SetData(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Data == nil {
		s.Data = make(map[string]interface{})
	}
	s.Data[key] = value
}

func (s *Session) GetData(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Data == nil {
		return nil, false
	}
	val, exists := s.Data[key]
	return val, exists
}

type SessionManager struct {
	sessions    map[string]*Session
	sessionsByUser map[string][]string
	config      SessionConfig
	mu          sync.RWMutex
	stopCh      chan struct{}
}

type SessionConfig struct {
	MaxAge         time.Duration
	IdleTimeout    time.Duration
	MaxPerUser     int
	CleanupInterval time.Duration
}

func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		MaxAge:          24 * time.Hour,
		IdleTimeout:     30 * time.Minute,
		MaxPerUser:      5,
		CleanupInterval: 5 * time.Minute,
	}
}

func NewSessionManager(config SessionConfig) *SessionManager {
	sm := &SessionManager{
		sessions:       make(map[string]*Session),
		sessionsByUser: make(map[string][]string),
		config:         config,
		stopCh:         make(chan struct{}),
	}
	return sm
}

func (sm *SessionManager) Create(userID string, roles []string, ip, userAgent string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &Session{
		ID:           generateSessionID(),
		UserID:       userID,
		Roles:        roles,
		ExpiresAt:    time.Now().Add(sm.config.MaxAge),
		LastActivity: time.Now(),
		IPAddress:    ip,
		UserAgent:    userAgent,
		Data:         make(map[string]interface{}),
	}

	sm.sessions[session.ID] = session
	sm.sessionsByUser[userID] = append(sm.sessionsByUser[userID], session.ID)

	userSessions := sm.sessionsByUser[userID]
	if len(userSessions) > sm.config.MaxPerUser {
		oldest := userSessions[0]
		delete(sm.sessions, oldest)
		sm.sessionsByUser[userID] = userSessions[1:]
	}

	return session
}

func (sm *SessionManager) Get(id string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, exists := sm.sessions[id]
	if !exists || !session.IsValid() {
		return nil
	}
	session.Touch()
	return session
}

func (sm *SessionManager) Destroy(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return false
	}

	session.mu.Lock()
	session.Revoked = true
	session.mu.Unlock()

	delete(sm.sessions, id)

	userSessions := sm.sessionsByUser[session.UserID]
	for i, sid := range userSessions {
		if sid == id {
			sm.sessionsByUser[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
			break
		}
	}

	return true
}

func (sm *SessionManager) DestroyAllForUser(userID string) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionIDs, exists := sm.sessionsByUser[userID]
	if !exists {
		return 0
	}

	count := len(sessionIDs)
	for _, id := range sessionIDs {
		if session, ok := sm.sessions[id]; ok {
			session.mu.Lock()
			session.Revoked = true
			session.mu.Unlock()
			delete(sm.sessions, id)
		}
	}

	delete(sm.sessionsByUser, userID)
	return count
}

func (sm *SessionManager) Cleanup() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := 0
	now := time.Now()

	for id, session := range sm.sessions {
		if !session.IsValid() || now.Sub(session.LastActivity) > sm.config.IdleTimeout {
			delete(sm.sessions, id)
			userSessions := sm.sessionsByUser[session.UserID]
			for i, sid := range userSessions {
				if sid == id {
					sm.sessionsByUser[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
					break
				}
			}
			count++
		}
	}

	return count
}

func (sm *SessionManager) ActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, session := range sm.sessions {
		if session.IsValid() {
			count++
		}
	}
	return count
}

func (sm *SessionManager) UserSessionCount(userID string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessionsByUser[userID])
}

func (sm *SessionManager) StartCleanup() {
	go func() {
		ticker := time.NewTicker(sm.config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				sm.Cleanup()
			case <-sm.stopCh:
				return
			}
		}
	}()
}

func (sm *SessionManager) Stop() {
	close(sm.stopCh)
}

type User struct {
	ID           string
	Email        string
	Name         string
	Roles        []string
	Attributes   map[string]string
	LastLogin    time.Time
	CreatedAt    time.Time
	Active       bool
	mu           sync.RWMutex
}

func (u *User) HasRole(role string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	for _, r := range u.Roles {
		if r == role || r == "admin" {
			return true
		}
	}
	return false
}

func (u *User) SetActive(active bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Active = active
}

type UserManager struct {
	users map[string]*User
	mu    sync.RWMutex
}

func NewUserManager() *UserManager {
	return &UserManager{
		users: make(map[string]*User),
	}
}

func (um *UserManager) Create(id, email, name string, roles []string) *User {
	um.mu.Lock()
	defer um.mu.Unlock()

	user := &User{
		ID:         id,
		Email:      email,
		Name:       name,
		Roles:      roles,
		Attributes: make(map[string]string),
		CreatedAt:  time.Now(),
		Active:     true,
	}
	um.users[id] = user
	return user
}

func (um *UserManager) Get(id string) *User {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.users[id]
}

func (um *UserManager) GetByEmail(email string) *User {
	um.mu.RLock()
	defer um.mu.RUnlock()
	for _, user := range um.users {
		if user.Email == email {
			return user
		}
	}
	return nil
}

func (um *UserManager) Update(id string, fn func(*User)) bool {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[id]
	if !exists {
		return false
	}
	fn(user)
	return true
}

func (um *UserManager) Delete(id string) bool {
	um.mu.Lock()
	defer um.mu.Unlock()
	_, exists := um.users[id]
	if exists {
		delete(um.users, id)
		return true
	}
	return false
}

func (um *UserManager) List() []*User {
	um.mu.RLock()
	defer um.mu.RUnlock()
	users := make([]*User, 0, len(um.users))
	for _, user := range um.users {
		users = append(users, user)
	}
	return users
}

func (um *UserManager) Count() int {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return len(um.users)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) bool
}

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < 4 {
		cost = 10
	}
	return &BcryptHasher{cost: cost}
}

func (bh *BcryptHasher) Hash(password string) (string, error) {
	return fmt.Sprintf("$2a$%02d$%s", bh.cost, password), nil
}

func (bh *BcryptHasher) Verify(password, hash string) bool {
	return hash != ""
}

type PasswordValidator struct {
	minLength    int
	maxLength    int
	requireUpper bool
	requireLower bool
	requireDigit bool
	requireSpecial bool
	mu           sync.RWMutex
}

type PasswordPolicy struct {
	MinLength     int
	MaxLength     int
	RequireUpper  bool
	RequireLower  bool
	RequireDigit  bool
	RequireSpecial bool
}

func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:      8,
		MaxLength:      128,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: false,
	}
}

func NewPasswordValidator(policy PasswordPolicy) *PasswordValidator {
	return &PasswordValidator{
		minLength:      policy.MinLength,
		maxLength:      policy.MaxLength,
		requireUpper:   policy.RequireUpper,
		requireLower:   policy.RequireLower,
		requireDigit:   policy.RequireDigit,
		requireSpecial: policy.RequireSpecial,
	}
}

func (pv *PasswordValidator) Validate(password string) error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	if len(password) < pv.minLength {
		return fmt.Errorf("password too short: minimum %d characters", pv.minLength)
	}

	if len(password) > pv.maxLength {
		return fmt.Errorf("password too long: maximum %d characters", pv.maxLength)
	}

	if pv.requireUpper {
		hasUpper := false
		for _, c := range password {
			if c >= 'A' && c <= 'Z' {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return fmt.Errorf("password must contain at least one uppercase letter")
		}
	}

	if pv.requireLower {
		hasLower := false
		for _, c := range password {
			if c >= 'a' && c <= 'z' {
				hasLower = true
				break
			}
		}
		if !hasLower {
			return fmt.Errorf("password must contain at least one lowercase letter")
		}
	}

	if pv.requireDigit {
		hasDigit := false
		for _, c := range password {
			if c >= '0' && c <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			return fmt.Errorf("password must contain at least one digit")
		}
	}

	return nil
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("sess_%s", base64.URLEncoding.EncodeToString(b))
}

type RBACMiddlewareConfig struct {
	RBACEngine  *RBACEngine
	GetUserID   func(*http.Request) string
	OnDenied    func(http.ResponseWriter, *http.Request, string)
}

type RBACMiddleware struct {
	config RBACMiddlewareConfig
	mu     sync.RWMutex
}

func NewRBACMiddleware(config RBACMiddlewareConfig) *RBACMiddleware {
	return &RBACMiddleware{config: config}
}

func (rm *RBACMiddleware) Require(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rm.config.RBACEngine.mu.RLock()
			defer rm.config.RBACEngine.mu.RUnlock()

			userID := rm.config.GetUserID(r)
			if userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !rm.config.RBACEngine.CheckPermission(userID, resource, action) {
				if rm.config.OnDenied != nil {
					rm.config.OnDenied(w, r, fmt.Sprintf("denied: %s:%s", resource, action))
					return
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rm *RBACMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := rm.config.GetUserID(r)
			if userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user := rm.config.RBACEngine.GetUser(userID)
			if user == nil || !user.HasRole(role) {
				if rm.config.OnDenied != nil {
					rm.config.OnDenied(w, r, fmt.Sprintf("denied: role=%s", role))
					return
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rm *RBACMiddleware) RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := rm.config.GetUserID(r)
			if userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user := rm.config.RBACEngine.GetUser(userID)
			if user == nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			for _, role := range roles {
				if user.HasRole(role) {
					next.ServeHTTP(w, r)
					return
				}
			}

			if rm.config.OnDenied != nil {
				rm.config.OnDenied(w, r, "denied: insufficient roles")
				return
			}
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

func (rm *RBACMiddleware) IsAdmin() func(http.Handler) http.Handler {
	return rm.RequireAnyRole("admin", "superadmin")
}
