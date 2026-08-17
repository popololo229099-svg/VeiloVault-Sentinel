package router

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type NestedRoute struct {
	Path       string
	Handler    http.HandlerFunc
	Middlewares []Middleware
	Children   []*NestedRoute
	Parent     *NestedRoute
	Depth      int
	Name       string
	Meta       map[string]interface{}
	mu         sync.RWMutex
}

type NestedRouter struct {
	root      *NestedRoute
	routes    map[string]*NestedRoute
	mu        sync.RWMutex
	metadata  map[string]interface{}
}

func NewNestedRouter() *NestedRouter {
	root := &NestedRoute{
		Path:     "/",
		Children: make([]*NestedRoute, 0),
		Depth:    0,
		Name:     "root",
		Meta:     make(map[string]interface{}),
	}
	return &NestedRouter{
		root:     root,
		routes:   make(map[string]*NestedRoute),
		metadata: make(map[string]interface{}),
	}
}

func (nr *NestedRouter) AddRoute(path string, handler http.HandlerFunc, middlewares ...Middleware) *NestedRoute {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	segments := strings.Split(strings.Trim(path, "/"), "/")
	current := nr.root

	for i, segment := range segments {
		found := false
		for _, child := range current.Children {
			if child.Name == segment {
				current = child
				found = true
				break
			}
		}

		if !found {
			newRoute := &NestedRoute{
				Path:       "/" + strings.Join(segments[:i+1], "/"),
				Children:   make([]*NestedRoute, 0),
				Parent:     current,
				Depth:      i + 1,
				Name:       segment,
				Meta:       make(map[string]interface{}),
			}
			current.Children = append(current.Children, newRoute)
			current = newRoute
		}
	}

	current.Handler = handler
	current.Middlewares = middlewares
	nr.routes[path] = current
	return current
}

func (nr *NestedRouter) AddGroup(path string, middlewares ...Middleware) *NestedRouteGroup {
	return &NestedRouteGroup{
		router:     nr,
		prefix:     path,
		middlewares: middlewares,
	}
}

func (nr *NestedRouter) FindRoute(path string) *NestedRoute {
	nr.mu.RLock()
	defer nr.mu.RUnlock()

	segments := strings.Split(strings.Trim(path, "/"), "/")
	return nr.findRouteRecursive(nr.root, segments, 0)
}

func (nr *NestedRouter) findRouteRecursive(node *NestedRoute, segments []string, depth int) *NestedRoute {
	if depth >= len(segments) {
		if node.Handler != nil {
			return node
		}
		return nil
	}

	for _, child := range node.Children {
		if child.Name == segments[depth] {
			result := nr.findRouteRecursive(child, segments, depth+1)
			if result != nil {
				return result
			}
		}
	}

	if node.Handler != nil && depth == len(segments) {
		return node
	}

	return nil
}

func (nr *NestedRouter) Traverse(fn func(*NestedRoute)) {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	nr.traverseNode(nr.root, fn)
}

func (nr *NestedRouter) traverseNode(node *NestedRoute, fn func(*NestedRoute)) {
	fn(node)
	for _, child := range node.Children {
		nr.traverseNode(child, fn)
	}
}

func (nr *NestedRouter) AllRoutes() []*NestedRoute {
	routes := make([]*NestedRoute, 0)
	nr.Traverse(func(node *NestedRoute) {
		if node.Handler != nil {
			routes = append(routes, node)
		}
	})
	return routes
}

func (nr *NestedRouter) MaxDepth() int {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	return nr.maxDepthRecursive(nr.root, 0)
}

func (nr *NestedRouter) maxDepthRecursive(node *NestedRoute, depth int) int {
	maxD := depth
	for _, child := range node.Children {
		d := nr.maxDepthRecursive(child, depth+1)
		if d > maxD {
			maxD = d
		}
	}
	return maxD
}

func (nr *NestedRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	route := nr.FindRoute(req.URL.Path)
	if route == nil || route.Handler == nil {
		http.NotFound(w, req)
		return
	}

	var h http.Handler = route.Handler
	for i := len(route.Middlewares) - 1; i >= 0; i-- {
		h = route.Middlewares[i](h)
	}

	h.ServeHTTP(w, req)
}

type NestedRouteGroup struct {
	router      *NestedRouter
	prefix      string
	middlewares []Middleware
	mu          sync.RWMutex
}

func (nrg *NestedRouteGroup) GET(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	nrg.Handle(http.MethodGet, path, handler, middlewares...)
}

func (nrg *NestedRouteGroup) POST(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	nrg.Handle(http.MethodPost, path, handler, middlewares...)
}

func (nrg *NestedRouteGroup) PUT(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	nrg.Handle(http.MethodPut, path, handler, middlewares...)
}

func (nrg *NestedRouteGroup) DELETE(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	nrg.Handle(http.MethodDelete, path, handler, middlewares...)
}

func (nrg *NestedRouteGroup) Handle(method, path string, handler http.HandlerFunc, middlewares ...Middleware) {
	nrg.mu.RLock()
	prefix := nrg.prefix
	mws := make([]Middleware, len(nrg.middlewares))
	copy(mws, nrg.middlewares)
	nrg.mu.RUnlock()

	fullPath := prefix + path
	allMws := make([]Middleware, 0, len(mws)+len(middlewares))
	allMws = append(allMws, mws...)
	allMws = append(allMws, middlewares...)

	wrappedHandler := func(w http.ResponseWriter, req *http.Request) {
		req.Header.Set("X-HTTP-Method", method)
		handler(w, req)
	}

	nrg.router.AddRoute(fullPath, wrappedHandler, allMws...)
}

func (nrg *NestedRouteGroup) Group(prefix string, middlewares ...Middleware) *NestedRouteGroup {
	nrg.mu.RLock()
	parentPrefix := nrg.prefix
	parentMws := make([]Middleware, len(nrg.middlewares))
	copy(parentMws, nrg.middlewares)
	nrg.mu.RUnlock()

	fullPrefix := parentPrefix + prefix
	allMws := make([]Middleware, 0, len(parentMws)+len(middlewares))
	allMws = append(allMws, parentMws...)
	allMws = append(allMws, middlewares...)

	return &NestedRouteGroup{
		router:      nrg.router,
		prefix:      fullPrefix,
		middlewares: allMws,
	}
}

type RouteTree struct {
	Root  *RouteTreeNode
	count int
	mu    sync.RWMutex
}

type RouteTreeNode struct {
	Name     string
	Path     string
	Handler  http.HandlerFunc
	Children map[string]*RouteTreeNode
	Parent   *RouteTreeNode
	Depth    int
	IsLeaf   bool
	Meta     map[string]interface{}
}

func NewRouteTree() *RouteTree {
	return &RouteTree{
		Root: &RouteTreeNode{
			Name:     "root",
			Path:     "/",
			Children: make(map[string]*RouteTreeNode),
			Meta:     make(map[string]interface{}),
		},
	}
}

func (rt *RouteTree) Insert(path string, handler http.HandlerFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	segments := strings.Split(strings.Trim(path, "/"), "/")
	current := rt.Root

	for i, segment := range segments {
		child, exists := current.Children[segment]
		if !exists {
			child = &RouteTreeNode{
				Name:     segment,
				Path:     "/" + strings.Join(segments[:i+1], "/"),
				Children: make(map[string]*RouteTreeNode),
				Parent:   current,
				Depth:    i + 1,
				Meta:     make(map[string]interface{}),
			}
			current.Children[segment] = child
		}
		current = child
	}

	current.Handler = handler
	current.IsLeaf = true
	rt.count++
}

func (rt *RouteTree) Search(path string) *RouteTreeNode {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	segments := strings.Split(strings.Trim(path, "/"), "/")
	current := rt.Root

	for _, segment := range segments {
		child, exists := current.Children[segment]
		if !exists {
			return nil
		}
		current = child
	}

	if current.IsLeaf {
		return current
	}
	return nil
}

func (rt *RouteTree) Delete(path string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	segments := strings.Split(strings.Trim(path, "/"), "/")
	return rt.deleteRecursive(rt.Root, segments, 0)
}

func (rt *RouteTree) deleteRecursive(node *RouteTreeNode, segments []string, depth int) bool {
	if depth >= len(segments) {
		if node.IsLeaf {
			node.IsLeaf = false
			node.Handler = nil
			rt.count--
			return true
		}
		return false
	}

	child, exists := node.Children[segments[depth]]
	if !exists {
		return false
	}

	if rt.deleteRecursive(child, segments, depth+1) {
		if len(child.Children) == 0 && !child.IsLeaf {
			delete(node.Children, segments[depth])
		}
		return true
	}
	return false
}

func (rt *RouteTree) List() []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	paths := make([]string, 0)
	rt.listRecursive(rt.Root, "", &paths)
	return paths
}

func (rt *RouteTree) listRecursive(node *RouteTreeNode, prefix string, paths *[]string) {
	if node.IsLeaf {
		path := prefix
		if path == "" {
			path = "/"
		}
		*paths = append(*paths, path)
	}

	for name, child := range node.Children {
		childPath := prefix + "/" + name
		rt.listRecursive(child, childPath, paths)
	}
}

func (rt *RouteTree) Count() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.count
}

func (rt *RouteTree) Depth() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.depthRecursive(rt.Root, 0)
}

func (rt *RouteTree) depthRecursive(node *RouteTreeNode, depth int) int {
	maxD := depth
	for _, child := range node.Children {
		d := rt.depthRecursive(child, depth+1)
		if d > maxD {
			maxD = d
		}
	}
	return maxD
}

type WildcardRoute struct {
	Pattern     string
	Handler     http.HandlerFunc
	Parts       []string
	HasWildcard bool
	WildcardPos int
	ParamNames  []string
}

type WildcardRouter struct {
	routes      []*WildcardRoute
	routesByKey map[string]*WildcardRoute
	mu          sync.RWMutex
}

func NewWildcardRouter() *WildcardRouter {
	return &WildcardRouter{
		routes:      make([]*WildcardRoute, 0),
		routesByKey: make(map[string]*WildcardRoute),
	}
}

func (wr *WildcardRouter) AddRoute(pattern string, handler http.HandlerFunc) {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	parts := strings.Split(strings.Trim(pattern, "/"), "/")
	hasWildcard := false
	wildcardPos := -1
	paramNames := make([]string, 0)

	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			paramNames = append(paramNames, part[1:])
		} else if strings.HasSuffix(part, "*") {
			hasWildcard = true
			wildcardPos = i
			name := strings.TrimSuffix(part, "*")
			if name != "" {
				paramNames = append(paramNames, name)
			}
		}
	}

	route := &WildcardRoute{
		Pattern:     pattern,
		Handler:     handler,
		Parts:       parts,
		HasWildcard: hasWildcard,
		WildcardPos: wildcardPos,
		ParamNames:  paramNames,
	}

	wr.routes = append(wr.routes, route)
	wr.routesByKey[pattern] = route
}

func (wr *WildcardRouter) Match(path string) (*WildcardRoute, map[string]string) {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	for _, route := range wr.routes {
		params := wr.matchRoute(route, pathParts)
		if params != nil {
			return route, params
		}
	}

	return nil, nil
}

func (wr *WildcardRouter) matchRoute(route *WildcardRoute, pathParts []string) map[string]string {
	params := make(map[string]string)
	paramIdx := 0

	for i, part := range route.Parts {
		if strings.HasPrefix(part, ":") {
			if i >= len(pathParts) {
				return nil
			}
			params[route.ParamNames[paramIdx]] = pathParts[i]
			paramIdx++
		} else if strings.HasSuffix(part, "*") {
			if i < len(pathParts) {
				remaining := strings.Join(pathParts[i:], "/")
				if len(route.ParamNames) > paramIdx {
					params[route.ParamNames[paramIdx]] = remaining
				}
			}
			return params
		} else {
			if i >= len(pathParts) || part != pathParts[i] {
				return nil
			}
		}
	}

	if len(pathParts) > len(route.Parts) && !route.HasWildcard {
		return nil
	}

	return params
}

func (wr *WildcardRouter) RemoveRoute(pattern string) bool {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	for i, route := range wr.routes {
		if route.Pattern == pattern {
			wr.routes = append(wr.routes[:i], wr.routes[i+1:]...)
			delete(wr.routesByKey, pattern)
			return true
		}
	}
	return false
}

func (wr *WildcardRouter) RouteCount() int {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	return len(wr.routes)
}

func (wr *WildcardRouter) SortBySpecificity() {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	sort.Slice(wr.routes, func(i, j int) bool {
		if len(wr.routes[i].Parts) != len(wr.routes[j].Parts) {
			return len(wr.routes[i].Parts) > len(wr.routes[j].Parts)
		}
		return !wr.routes[i].HasWildcard && wr.routes[j].HasWildcard
	})
}

type RegexRoute struct {
	Pattern   string
	Regexp    string
	Handler   http.HandlerFunc
	compiled  interface{}
	ParamDefs []ParamDef
	mu        sync.RWMutex
}

type ParamDef struct {
	Name     string
	Regex    string
	Required bool
}

type RegexRouter struct {
	routes []*RegexRoute
	mu     sync.RWMutex
}

func NewRegexRouter() *RegexRouter {
	return &RegexRouter{
		routes: make([]*RegexRoute, 0),
	}
}

func (rr *RegexRouter) AddRoute(pattern, regex string, handler http.HandlerFunc) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.routes = append(rr.routes, &RegexRoute{
		Pattern: pattern,
		Regexp:  regex,
		Handler: handler,
	})
}

func (rr *RegexRouter) Match(path string) *RegexRoute {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	for _, route := range rr.routes {
		if route.Regexp == path {
			return route
		}
	}
	return nil
}

func (rr *RegexRouter) RouteCount() int {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return len(rr.routes)
}

func (rr *RegexRouter) ListRoutes() []string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	patterns := make([]string, len(rr.routes))
	for i, route := range rr.routes {
		patterns[i] = route.Pattern
	}
	return patterns
}

type RouteRanker struct {
	routes map[string]int
	mu     sync.RWMutex
}

func NewRouteRanker() *RouteRanker {
	return &RouteRanker{
		routes: make(map[string]int),
	}
}

func (rr *RouteRanker) Rank(path string, score int) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.routes[path] = score
}

func (rr *RouteRanker) Top(n int) []string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	type pathScore struct {
		path  string
		score int
	}

	scores := make([]pathScore, 0, len(rr.routes))
	for path, score := range rr.routes {
		scores = append(scores, pathScore{path, score})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	result := make([]string, 0, n)
	for i := 0; i < n && i < len(scores); i++ {
		result = append(result, scores[i].path)
	}
	return result
}

type PathPatternMatcher struct {
	patterns map[string]string
	mu       sync.RWMutex
}

func NewPathPatternMatcher() *PathPatternMatcher {
	return &PathPatternMatcher{
		patterns: make(map[string]string),
	}
}

func (ppm *PathPatternMatcher) Register(name, pattern string) {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()
	ppm.patterns[name] = pattern
}

func (ppm *PathPatternMatcher) Match(path string) (string, bool) {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()

	for name, pattern := range ppm.patterns {
		if pattern == path || pattern == "*" {
			return name, true
		}
	}
	return "", false
}

func (ppm *PathPatternMatcher) ListPatterns() map[string]string {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range ppm.patterns {
		result[k] = v
	}
	return result
}

type MethodNotAllowedHandler struct {
	allowed map[string][]string
	mu      sync.RWMutex
}

func NewMethodNotAllowedHandler() *MethodNotAllowedHandler {
	return &MethodNotAllowedHandler{
		allowed: make(map[string][]string),
	}
}

func (mnah *MethodNotAllowedHandler) Register(path string, methods []string) {
	mnah.mu.Lock()
	defer mnah.mu.Unlock()
	mnah.allowed[path] = methods
}

func (mnah *MethodNotAllowedHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	mnah.mu.RLock()
	defer mnah.mu.RUnlock()

	methods, exists := mnah.allowed[req.URL.Path]
	if exists {
		w.Header().Set("Allow", strings.Join(methods, ", "))
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	http.NotFound(w, req)
}

func (mnah *MethodNotAllowedHandler) IsAllowed(path, method string) bool {
	mnah.mu.RLock()
	defer mnah.mu.RUnlock()

	methods, exists := mnah.allowed[path]
	if !exists {
		return false
	}
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

func BuildRoutePath(parts ...string) string {
	result := ""
	for _, part := range parts {
		if result == "" {
			result = part
		} else {
			result = result + "/" + part
		}
	}
	return "/" + strings.Trim(result, "/")
}

func NormalizePath(path string) string {
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	return path
}

func JoinPaths(base string, paths ...string) string {
	result := strings.TrimRight(base, "/")
	for _, p := range paths {
		result = result + "/" + strings.Trim(p, "/")
	}
	return result
}

func SplitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

func PathDepth(path string) int {
	return len(SplitPath(path))
}

func PathParent(path string) string {
	parts := SplitPath(path)
	if len(parts) <= 1 {
		return "/"
	}
	return "/" + strings.Join(parts[:len(parts)-1], "/")
}

func IsChildPath(parent, child string) bool {
	parent = NormalizePath(parent)
	child = NormalizePath(child)
	return strings.HasPrefix(child, parent+"/") && child != parent
}

func CommonAncestor(path1, path2 string) string {
	parts1 := SplitPath(path1)
	parts2 := SplitPath(path2)

	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	common := make([]string, 0)
	for i := 0; i < minLen; i++ {
		if parts1[i] == parts2[i] {
			common = append(common, parts1[i])
		} else {
			break
		}
	}

	if len(common) == 0 {
		return "/"
	}
	return "/" + strings.Join(common, "/")
}

func PathDiff(base, full string) string {
	base = NormalizePath(base)
	full = NormalizePath(full)

	if !strings.HasPrefix(full, base) {
		return full
	}

	diff := strings.TrimPrefix(full, base)
	return strings.Trim(diff, "/")
}

func SortPaths(paths []string) []string {
	result := make([]string, len(paths))
	copy(result, paths)
	sort.Strings(result)
	return result
}

func UniquePaths(paths []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, p := range paths {
		p = NormalizePath(p)
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

func PathMatches(path, pattern string) bool {
	pathParts := SplitPath(path)
	patternParts := SplitPath(pattern)

	if len(patternParts) == 1 && patternParts[0] == "*" {
		return true
	}

	if len(pathParts) != len(patternParts) {
		if len(patternParts) == 0 || patternParts[len(patternParts)-1] != "*" {
			return false
		}
	}

	for i, pp := range patternParts {
		if pp == "*" {
			return true
		}
		if pp == ":" {
			continue
		}
		if i >= len(pathParts) || pathParts[i] != pp {
			return false
		}
	}

	return true
}

func ExtractWildcardParams(path, pattern string) map[string]string {
	params := make(map[string]string)
	pathParts := SplitPath(path)
	patternParts := SplitPath(pattern)

	for i, pp := range patternParts {
		if strings.HasPrefix(pp, ":") {
			paramName := pp[1:]
			if i < len(pathParts) {
				params[paramName] = pathParts[i]
			}
		} else if strings.HasSuffix(pp, "*") {
			prefix := strings.TrimSuffix(pp, "*")
			if prefix == "" {
				params["*"] = strings.Join(pathParts[i:], "/")
			}
		}
	}

	return params
}

func FormatRoutePath(format string, args ...interface{}) string {
	result := fmt.Sprintf(format, args...)
	return NormalizePath(result)
}

type PathEncoder interface {
	Encode(path string) string
	Decode(path string) string
}

type NoOpPathEncoder struct{}

func (NoOpPathEncoder) Encode(path string) string { return path }
func (NoOpPathEncoder) Decode(path string) string { return path }

type PathValidator interface {
	Validate(path string) error
}

type RegexPathValidator struct {
	patterns map[string]*regexp.Regexp
	mu       sync.RWMutex
}

func NewRegexPathValidator() *RegexPathValidator {
	return &RegexPathValidator{
		patterns: make(map[string]*regexp.Regexp),
	}
}

func (rpv *RegexPathValidator) AddPattern(name, pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	rpv.mu.Lock()
	defer rpv.mu.Unlock()
	rpv.patterns[name] = compiled
	return nil
}

func (rpv *RegexPathValidator) Validate(path string) error {
	rpv.mu.RLock()
	defer rpv.mu.RUnlock()

	for _, compiled := range rpv.patterns {
		if compiled.MatchString(path) {
			return nil
		}
	}
	return fmt.Errorf("path does not match any pattern: %s", path)
}

func (rpv *RegexPathValidator) Matches(path, patternName string) bool {
	rpv.mu.RLock()
	defer rpv.mu.RUnlock()

	compiled, exists := rpv.patterns[patternName]
	if !exists {
		return false
	}
	return compiled.MatchString(path)
}
