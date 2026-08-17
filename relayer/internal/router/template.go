package router

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type URLTemplate struct {
	raw      string
	parts    []TemplatePart
	compiled *regexp.Regexp
	mu       sync.RWMutex
}

type TemplatePart struct {
	Name     string
	IsParam  bool
	IsWild   bool
	Static   string
	Regex    string
	Required bool
	Default  string
}

type TemplateConfig struct {
	Separator      string
	ParamPrefix    string
	WildcardSuffix string
	EnableDefaults bool
	EnableRegex    bool
}

type TemplateMatcher struct {
	templates map[string]*URLTemplate
	cache     map[string]*TemplateMatchResult
	mu        sync.RWMutex
	config    TemplateConfig
}

type TemplateMatchResult struct {
	Template *URLTemplate
	Params   map[string]string
	Matched  bool
}

var defaultTemplateConfig = TemplateConfig{
	Separator:      "/",
	ParamPrefix:    ":",
	WildcardSuffix: "*",
	EnableDefaults: true,
	EnableRegex:    true,
}

func NewURLTemplate(raw string) (*URLTemplate, error) {
	return NewURLTemplateWithConfig(raw, defaultTemplateConfig)
}

func NewURLTemplateWithConfig(raw string, config TemplateConfig) (*URLTemplate, error) {
	t := &URLTemplate{
		raw:   raw,
		parts: make([]TemplatePart, 0),
	}

	if err := t.parse(config); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *URLTemplate) parse(config TemplateConfig) error {
	path := strings.TrimPrefix(t.raw, config.Separator)
	path = strings.TrimSuffix(path, config.Separator)

	if path == "" {
		return nil
	}

	segments := strings.Split(path, config.Separator)

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if strings.HasPrefix(segment, config.ParamPrefix) {
			paramName := strings.TrimPrefix(segment, config.ParamPrefix)
			required := !strings.HasSuffix(paramName, "?")
			paramName = strings.TrimSuffix(paramName, "?")

			defaultVal := ""
			if config.EnableDefaults {
				parts := strings.SplitN(paramName, "=", 2)
				if len(parts) == 2 {
					paramName = parts[0]
					defaultVal = parts[1]
				}
			}

			if config.EnableRegex && strings.Contains(paramName, "{") {
				regexParts := regexp.MustCompile(`\{([^}]+)\}`).FindStringSubmatch(paramName)
				if len(regexParts) > 1 {
					t.parts = append(t.parts, TemplatePart{
						Name:     strings.Split(paramName, "{")[0],
						IsParam:  true,
						Regex:    regexParts[1],
						Required: required,
						Default:  defaultVal,
					})
					continue
				}
			}

			t.parts = append(t.parts, TemplatePart{
				Name:     paramName,
				IsParam:  true,
				Required: required,
				Default:  defaultVal,
			})
		} else if strings.HasSuffix(segment, config.WildcardSuffix) {
			paramName := strings.TrimSuffix(segment, config.WildcardSuffix)
			if paramName == "" {
				paramName = "*"
			}
			t.parts = append(t.parts, TemplatePart{
				Name:    paramName,
				IsWild:  true,
				IsParam: true,
			})
		} else {
			t.parts = append(t.parts, TemplatePart{
				Static: segment,
			})
		}
	}

	return nil
}

func (t *URLTemplate) Match(path string) *TemplateMatchResult {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := &TemplateMatchResult{
		Template: t,
		Params:   make(map[string]string),
		Matched:  false,
	}

	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")

	partIdx := 0
	segIdx := 0

	for partIdx < len(t.parts) && segIdx < len(segments) {
		part := t.parts[partIdx]

		if part.IsWild {
			remaining := strings.Join(segments[segIdx:], "/")
			result.Params[part.Name] = remaining
			result.Matched = true
			return result
		}

		if part.IsParam {
			result.Params[part.Name] = segments[segIdx]
			segIdx++
			partIdx++
		} else {
			if part.Static != segments[segIdx] {
				return result
			}
			segIdx++
			partIdx++
		}
	}

	if partIdx < len(t.parts) {
		for ; partIdx < len(t.parts); partIdx++ {
			part := t.parts[partIdx]
			if part.IsParam && part.Default != "" {
				result.Params[part.Name] = part.Default
			} else if part.IsParam && part.Required {
				return result
			}
		}
	}

	if segIdx < len(segments) {
		return result
	}

	result.Matched = true
	return result
}

func (t *URLTemplate) Build(params map[string]string) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var sb strings.Builder
	for i, part := range t.parts {
		if i > 0 {
			sb.WriteString("/")
		}

		if part.IsParam {
			val, exists := params[part.Name]
			if !exists && part.Default != "" {
				val = part.Default
			} else if !exists && part.Required {
				return "", fmt.Errorf("missing required param: %s", part.Name)
			}
			sb.WriteString(val)
		} else {
			sb.WriteString(part.Static)
		}
	}

	return sb.String(), nil
}

func (t *URLTemplate) ParamNames() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	names := make([]string, 0)
	for _, part := range t.parts {
		if part.IsParam {
			names = append(names, part.Name)
		}
	}
	return names
}

func (t *URLTemplate) Raw() string {
	return t.raw
}

func (t *URLTemplate) String() string {
	return t.raw
}

func NewTemplateMatcher(config ...TemplateConfig) *TemplateMatcher {
	cfg := defaultTemplateConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	return &TemplateMatcher{
		templates: make(map[string]*URLTemplate),
		cache:     make(map[string]*TemplateMatchResult),
		config:    cfg,
	}
}

func (tm *TemplateMatcher) AddTemplate(name string, raw string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tmpl, err := NewURLTemplateWithConfig(raw, tm.config)
	if err != nil {
		return err
	}
	tm.templates[name] = tmpl
	return nil
}

func (tm *TemplateMatcher) Match(path string) *TemplateMatchResult {
	tm.mu.RLock()
	if cached, exists := tm.cache[path]; exists {
		tm.mu.RUnlock()
		return cached
	}
	tm.mu.RUnlock()

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, tmpl := range tm.templates {
		result := tmpl.Match(path)
		if result.Matched {
			tm.mu.RUnlock()
			tm.mu.Lock()
			tm.cache[path] = result
			tm.mu.Unlock()
			tm.mu.RLock()
			return result
		}
	}

	return &TemplateMatchResult{Matched: false}
}

func (tm *TemplateMatcher) ClearCache() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.cache = make(map[string]*TemplateMatchResult)
}

func (tm *TemplateMatcher) TemplateCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.templates)
}

type RouteComparator struct {
	templates []*URLTemplate
	mu        sync.RWMutex
}

func NewRouteComparator() *RouteComparator {
	return &RouteComparator{
		templates: make([]*URLTemplate, 0),
	}
}

func (rc *RouteComparator) AddRoute(path string) error {
	tmpl, err := NewURLTemplate(path)
	if err != nil {
		return err
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.templates = append(rc.templates, tmpl)
	return nil
}

func (rc *RouteComparator) FindMatching(path string) []*URLTemplate {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	matches := make([]*URLTemplate, 0)
	for _, tmpl := range rc.templates {
		result := tmpl.Match(path)
		if result.Matched {
			matches = append(matches, tmpl)
		}
	}
	return matches
}

func (rc *RouteComparator) Conflicts(path1, path2 string) bool {
	tmpl1, err := NewURLTemplate(path1)
	if err != nil {
		return false
	}
	tmpl2, err := NewURLTemplate(path2)
	if err != nil {
		return false
	}

	testPaths := []string{
		"/test/123",
		"/test/abc",
		"/test/hello/world",
		"/",
	}

	overlapCount := 0
	for _, tp := range testPaths {
		r1 := tmpl1.Match(tp)
		r2 := tmpl2.Match(tp)
		if r1.Matched && r2.Matched {
			overlapCount++
		}
	}

	return overlapCount > 0
}

type PathNormalizer struct {
	slashPolicy    string
	casePolicy     string
	trimExtensions bool
	mu             sync.RWMutex
}

type PathNormalizerConfig struct {
	SlashPolicy    string
	CasePolicy     string
	TrimExtensions bool
}

func NewPathNormalizer(config PathNormalizerConfig) *PathNormalizer {
	return &PathNormalizer{
		slashPolicy:    config.SlashPolicy,
		casePolicy:     config.CasePolicy,
		trimExtensions: config.TrimExtensions,
	}
}

func (pn *PathNormalizer) Normalize(path string) string {
	pn.mu.RLock()
	defer pn.mu.RUnlock()

	switch pn.slashPolicy {
	case "merge":
		for strings.Contains(path, "//") {
			path = strings.ReplaceAll(path, "//", "/")
		}
	case "trim_trailing":
		path = strings.TrimRight(path, "/")
	case "trim_leading":
		path = strings.TrimLeft(path, "/")
	case "trim_both":
		path = strings.Trim(path, "/")
	}

	switch pn.casePolicy {
	case "lower":
		path = strings.ToLower(path)
	case "upper":
		path = strings.ToUpper(path)
	}

	if pn.trimExtensions {
		if idx := strings.LastIndex(path, "."); idx > strings.LastIndex(path, "/") {
			path = path[:idx]
		}
	}

	return path
}

func (pn *PathNormalizer) Equals(path1, path2 string) bool {
	return pn.Normalize(path1) == pn.Normalize(path2)
}

type PathBuilder struct {
	segments []string
	params   map[string]string
	mu       sync.RWMutex
}

func NewPathBuilder() *PathBuilder {
	return &PathBuilder{
		segments: make([]string, 0),
		params:   make(map[string]string),
	}
}

func (pb *PathBuilder) Segment(s string) *PathBuilder {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.segments = append(pb.segments, s)
	return pb
}

func (pb *PathBuilder) Param(name, value string) *PathBuilder {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.segments = append(pb.segments, ":"+name)
	pb.params[name] = value
	return pb
}

func (pb *PathBuilder) Wildcard(name string) *PathBuilder {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.segments = append(pb.segments, "*"+name)
	return pb
}

func (pb *PathBuilder) Build() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	parts := make([]string, len(pb.segments))
	copy(parts, pb.segments)

	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") {
			name := seg[1:]
			if val, exists := pb.params[name]; exists {
				parts[i] = val
			}
		}
	}

	return "/" + strings.Join(parts, "/")
}

func (pb *PathBuilder) BuildTemplate() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return "/" + strings.Join(pb.segments, "/")
}

func (pb *PathBuilder) Params() map[string]string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range pb.params {
		result[k] = v
	}
	return result
}
