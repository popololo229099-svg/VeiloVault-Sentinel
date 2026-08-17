package router

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

type Route struct {
	Method      string
	Path        string
	Handler     http.HandlerFunc
	Middlewares []Middleware
	Params      map[string]string
	Regex       *regexp.Regexp
	IsWildcard  bool
	Group       *RouteGroup
	Meta        map[string]interface{}
}

type Middleware func(http.Handler) http.Handler

type RouteGroup struct {
	Name        string
	Prefix      string
	Routes      []*Route
	Groups      []*RouteGroup
	Middlewares []Middleware
	Parent      *RouteGroup
	Router      *Router
	mu          sync.RWMutex
}

type Router struct {
	routes      []*Route
	groups      []*RouteGroup
	trees       map[string]*RouteTrie
	notFound    http.HandlerFunc
	methodNotAllowed http.HandlerFunc
	mu          sync.RWMutex
	middlewares []Middleware
	maxParams   int
	paramPool   sync.Pool
}

type RouteTrie struct {
	Children   map[string]*RouteTrie
	ParamChild *RouteTrie
	WildChild  *RouteTrie
	ParamName  string
	Route      *Route
	IsLeaf     bool
}

type RouterConfig struct {
	MaxParams    int
	RedirectTrailingSlash bool
	RedirectFixedPath     bool
	HandleMethodNotAllowed bool
	RemoveExtraSlash      bool
}

type RouteMatch struct {
	Route    *Route
	Params   map[string]string
	Handler  http.HandlerFunc
	Found    bool
}

func NewRouter() *Router {
	return &Router{
		routes:  make([]*Route, 0),
		groups:  make([]*RouteGroup, 0),
		trees:   make(map[string]*RouteTrie),
		maxParams: 64,
		paramPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]string)
			},
		},
	}
}

func NewRouterWithConfig(config RouterConfig) *Router {
	r := NewRouter()
	if config.MaxParams > 0 {
		r.maxParams = config.MaxParams
	}
	return r
}

func (r *Router) getParams() map[string]string {
	return r.paramPool.Get().(map[string]string)
}

func (r *Router) putParams(params map[string]string) {
	for k := range params {
		delete(params, k)
	}
	r.paramPool.Put(params)
}

func (r *Router) GET(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.Handle(http.MethodGet, path, handler, middlewares...)
}

func (r *Router) POST(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.Handle(http.MethodPost, path, handler, middlewares...)
}

func (r *Router) PUT(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.Handle(http.MethodPut, path, handler, middlewares...)
}

func (r *Router) DELETE(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.Handle(http.MethodDelete, path, handler, middlewares...)
}

func (r *Router) PATCH(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.Handle(http.MethodPatch, path, handler, middlewares...)
}

func (r *Router) OPTIONS(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.Handle(http.MethodOptions, path, handler, middlewares...)
}

func (r *Router) HEAD(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.Handle(http.MethodHead, path, handler, middlewares...)
}

func (r *Router) Handle(method, path string, handler http.HandlerFunc, middlewares ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()

	route := &Route{
		Method:      method,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
		Params:      make(map[string]string),
		Meta:        make(map[string]interface{}),
	}

	if strings.Contains(path, "*") {
		route.IsWildcard = true
	}

	if strings.Contains(path, ":") {
		compiled, err := compilePath(path)
		if err == nil {
			route.Regex = compiled
		}
	}

	r.routes = append(r.routes, route)

	tree, exists := r.trees[method]
	if !exists {
		tree = &RouteTrie{
			Children: make(map[string]*RouteTrie),
		}
		r.trees[method] = tree
	}

	insertTrie(tree, path, route)
}

func compilePath(path string) (*regexp.Regexp, error) {
	regexPath := path
	regexPath = regexp.QuoteMeta(regexPath)
	regexPath = strings.ReplaceAll(regexPath, "\\:([a-zA-Z_][a-zA-Z0-9_]*)", "([^/]+)")
	regexPath = strings.ReplaceAll(regexPath, "\\*([a-zA-Z_]*)", "(.*)")
	return regexp.Compile("^" + regexPath + "$")
}

func insertTrie(root *RouteTrie, path string, route *Route) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	current := root
	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			if current.ParamChild == nil {
				current.ParamChild = &RouteTrie{
					Children:  make(map[string]*RouteTrie),
					ParamName: part[1:],
				}
			}
			current = current.ParamChild
		} else if strings.HasPrefix(part, "*") {
			if current.WildChild == nil {
				current.WildChild = &RouteTrie{
					Children:  make(map[string]*RouteTrie),
					ParamName: part[1:],
				}
			}
			current = current.WildChild
			current.Route = route
			current.IsLeaf = true
			return
		} else {
			if _, exists := current.Children[part]; !exists {
				current.Children[part] = &RouteTrie{
					Children: make(map[string]*RouteTrie),
				}
			}
			current = current.Children[part]
		}
	}

	current.Route = route
	current.IsLeaf = true
}

func (r *Router) Match(method, path string) *RouteMatch {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tree, exists := r.trees[method]
	if !exists {
		return &RouteMatch{Found: false}
	}

	params := r.getParams()
	result := matchTrie(tree, path, params)

	if result != nil && result.Route != nil {
		chain := r.buildHandlerChain(result.Route)
		return &RouteMatch{
			Route:   result.Route,
			Params:  params,
			Handler: chain,
			Found:   true,
		}
	}

	r.putParams(params)
	return &RouteMatch{Found: false}
}

func matchTrie(node *RouteTrie, path string, params map[string]string) *RouteTrie {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		if node.IsLeaf {
			return node
		}
		return nil
	}

	parts := strings.SplitN(path, "/", 2)
	current := parts[0]
	remaining := ""
	if len(parts) > 1 {
		remaining = parts[1]
	}

	if child, exists := node.Children[current]; exists {
		if result := matchTrie(child, remaining, params); result != nil {
			return result
		}
	}

	if node.ParamChild != nil {
		params[node.ParamChild.ParamName] = current
		if result := matchTrie(node.ParamChild, remaining, params); result != nil {
			return result
		}
		delete(params, node.ParamChild.ParamName)
	}

	if node.WildChild != nil {
		params[node.WildChild.ParamName] = path
		return node.WildChild
	}

	return nil
}

func (r *Router) buildHandlerChain(route *Route) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		params := r.getParams()
		match := matchTrie(r.trees[route.Method], route.Path, params)
		if match != nil && match.Route != nil {
			for k, v := range params {
				req.Header.Set("X-Param-"+k, v)
			}
		}
		r.putParams(params)

		var handler http.Handler = route.Handler
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		for i := len(r.middlewares) - 1; i >= 0; i-- {
			handler = r.middlewares[i](handler)
		}

		handler.ServeHTTP(w, req)
	}
}

func (r *Router) Use(middlewares ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *Router) Group(prefix string, middlewares ...Middleware) *RouteGroup {
	group := &RouteGroup{
		Prefix:      prefix,
		Routes:      make([]*Route, 0),
		Groups:      make([]*RouteGroup, 0),
		Middlewares: middlewares,
		Router:      r,
	}
	r.groups = append(r.groups, group)
	return group
}

func (rg *RouteGroup) GET(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	rg.Handle(http.MethodGet, path, handler, middlewares...)
}

func (rg *RouteGroup) POST(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	rg.Handle(http.MethodPost, path, handler, middlewares...)
}

func (rg *RouteGroup) PUT(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	rg.Handle(http.MethodPut, path, handler, middlewares...)
}

func (rg *RouteGroup) DELETE(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	rg.Handle(http.MethodDelete, path, handler, middlewares...)
}

func (rg *RouteGroup) PATCH(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	rg.Handle(http.MethodPatch, path, handler, middlewares...)
}

func (rg *RouteGroup) OPTIONS(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	rg.Handle(http.MethodOptions, path, handler, middlewares...)
}

func (rg *RouteGroup) HEAD(path string, handler http.HandlerFunc, middlewares ...Middleware) {
	rg.Handle(http.MethodHead, path, handler, middlewares...)
}

func (rg *RouteGroup) Handle(method, path string, handler http.HandlerFunc, middlewares ...Middleware) {
	fullPath := rg.Prefix + path
	allMiddlewares := make([]Middleware, 0, len(rg.Middlewares)+len(middlewares))
	allMiddlewares = append(allMiddlewares, rg.Middlewares...)
	allMiddlewares = append(allMiddlewares, middlewares...)
	rg.Router.Handle(method, fullPath, handler, allMiddlewares...)
}

func (rg *RouteGroup) Group(prefix string, middlewares ...Middleware) *RouteGroup {
	fullPrefix := rg.Prefix + prefix
	group := &RouteGroup{
		Prefix:      fullPrefix,
		Routes:      make([]*Route, 0),
		Groups:      make([]*RouteGroup, 0),
		Middlewares: middlewares,
		Parent:      rg,
		Router:      rg.Router,
	}
	rg.Groups = append(rg.Groups, group)
	return group
}

func (rg *RouteGroup) Use(middlewares ...Middleware) {
	rg.Middlewares = append(rg.Middlewares, middlewares...)
}

func (r *Router) NotFound(handler http.HandlerFunc) {
	r.notFound = handler
}

func (r *Router) MethodNotAllowed(handler http.HandlerFunc) {
	r.methodNotAllowed = handler
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	method := req.Method

	match := r.Match(method, path)

	if !match.Found {
		if r.notFound != nil {
			r.notFound(w, req)
		} else {
			http.NotFound(w, req)
		}
		return
	}

	match.Handler(w, req)
}

func (r *Router) Routes() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Route, len(r.routes))
	copy(result, r.routes)
	return result
}

func (r *Router) RouteCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes)
}

func (r *Router) MatchRoute(method, path string) *Route {
	match := r.Match(method, path)
	if match.Found {
		return match.Route
	}
	return nil
}

func (r *Router) GroupRoutes() []*RouteGroup {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*RouteGroup, len(r.groups))
	copy(result, r.groups)
	return result
}

func (r *Router) Reverse(route *Route, params map[string]string) string {
	path := route.Path
	for key, value := range params {
		path = strings.Replace(path, ":"+key, value, 1)
	}
	return path
}

func (r *Router) HasRoute(method, path string) bool {
	return r.MatchRoute(method, path) != nil
}

func (r *Router) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = make([]*Route, 0)
	r.groups = make([]*RouteGroup, 0)
	r.trees = make(map[string]*RouteTrie)
	r.middlewares = make([]Middleware, 0)
}

func (r *Router) Clone() *Router {
	r.mu.RLock()
	defer r.mu.RUnlock()

	newRouter := NewRouter()
	newRouter.routes = make([]*Route, len(r.routes))
	copy(newRouter.routes, r.routes)
	newRouter.middlewares = make([]Middleware, len(r.middlewares))
	copy(newRouter.middlewares, r.middlewares)
	return newRouter
}

func (r *Router) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Router:\n")
	for _, route := range r.routes {
		sb.WriteString(fmt.Sprintf("  %s %s\n", route.Method, route.Path))
	}
	return sb.String()
}

func ExtractParam(handler http.HandlerFunc, paramName string, req *http.Request) string {
	return req.Header.Get("X-Param-" + paramName)
}

func ParamsFromRequest(req *http.Request) map[string]string {
	params := make(map[string]string)
	for key, values := range req.Header {
		if strings.HasPrefix(key, "X-Param-") {
			paramName := strings.TrimPrefix(key, "X-Param-")
			if len(values) > 0 {
				params[paramName] = values[0]
			}
		}
	}
	return params
}

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, fmt.Sprintf("Internal Server Error: %v", err), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, req)
	})
}

func MethodFilter(methods ...string) Middleware {
	methodSet := make(map[string]bool)
	for _, m := range methods {
		methodSet[m] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !methodSet[req.Method] {
				w.Header().Set("Allow", strings.Join(methods, ", "))
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func PathPrefixMiddleware(prefix string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !strings.HasPrefix(req.URL.Path, prefix) {
				http.NotFound(w, req)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func ChainMiddlewares(handler http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	var h http.Handler = handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return func(w http.ResponseWriter, req *http.Request) {
		h.ServeHTTP(w, req)
	}
}
