package pattern

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPClient struct {
	client     *http.Client
	baseURL    string
	headers    map[string]string
	retryCount int
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		baseURL:    baseURL,
		headers:    make(map[string]string),
		retryCount: 3,
	}
}

func (c *HTTPClient) SetHeader(key, value string) {
	c.headers[key] = value
}

func (c *HTTPClient) SetRetryCount(n int) {
	c.retryCount = n
}

func (c *HTTPClient) Get(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

func (c *HTTPClient) Post(ctx context.Context, path string, body interface{}) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, data)
}

func (c *HTTPClient) Put(ctx context.Context, path string, body interface{}) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPut, path, data)
}

func (c *HTTPClient) Delete(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, path, nil)
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	url := c.baseURL + path
	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, bytesReader(body))
		if err != nil {
			return nil, err
		}
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}
		return data, nil
	}
	return nil, lastErr
}

func bytesReader(b []byte) io.Reader {
	if b == nil {
		return nil
	}
	return &byteReader{data: b, pos: 0}
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

type ResponseEnvelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Page      int `json:"page,omitempty"`
	PerPage   int `json:"per_page,omitempty"`
	Total     int `json:"total,omitempty"`
	TotalPage int `json:"total_page,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ResponseEnvelope{
		Success: status >= 200 && status < 300,
		Data:    data,
	})
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ResponseEnvelope{
		Success: false,
		Error:   msg,
	})
}

func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

func WriteCreated(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, data)
}

func ParseJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

type Router struct {
	routes  map[string]map[string]http.HandlerFunc
	mux     *http.ServeMux
	middlewares []func(http.Handler) http.Handler
}

func NewRouter() *Router {
	return &Router{
		routes: make(map[string]map[string]http.HandlerFunc),
		mux:    http.NewServeMux(),
	}
}

func (rt *Router) Use(middleware func(http.Handler) http.Handler) {
	rt.middlewares = append(rt.middlewares, middleware)
}

func (rt *Router) GET(path string, handler http.HandlerFunc) {
	rt.register("GET", path, handler)
}

func (rt *Router) POST(path string, handler http.HandlerFunc) {
	rt.register("POST", path, handler)
}

func (rt *Router) PUT(path string, handler http.HandlerFunc) {
	rt.register("PUT", path, handler)
}

func (rt *Router) DELETE(path string, handler http.HandlerFunc) {
	rt.register("DELETE", path, handler)
}

func (rt *Router) PATCH(path string, handler http.HandlerFunc) {
	rt.register("PATCH", path, handler)
}

func (rt *Router) register(method, path string, handler http.HandlerFunc) {
	if rt.routes[path] == nil {
		rt.routes[path] = make(map[string]http.HandlerFunc)
	}
	rt.routes[path][method] = handler
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := rt.routes[r.URL.Path][r.Method]; ok {
			h(w, r)
			return
		}
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	rt.mux.Handle(path, wrapped)
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := http.Handler(rt.mux)
	for i := len(rt.middlewares) - 1; i >= 0; i-- {
		handler = rt.middlewares[i](handler)
	}
	handler.ServeHTTP(w, r)
}

func (rt *Router) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, rt)
}
