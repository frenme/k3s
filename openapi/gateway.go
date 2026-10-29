package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

const (
	rulesFile = "internal/gateway/policy/rules_gen.go"
	specFile  = "internal/gateway/docs/openapi_gen.json"
)

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

type rule struct {
	method, path, access string
	timeout              time.Duration
	maxBody              int64
	cacheTTL             time.Duration
}

func writeGateway(specs map[string]*huma.OpenAPI) error {
	rules, err := collectRules(specs)
	if err != nil {
		return err
	}
	src, err := renderRules(rules)
	if err != nil {
		return err
	}
	if err := os.WriteFile(rulesFile, src, 0o600); err != nil {
		return err
	}
	merged, err := mergeSpecs(specs)
	if err != nil {
		return err
	}
	return os.WriteFile(specFile, merged, 0o600)
}

func collectRules(specs map[string]*huma.OpenAPI) ([]rule, error) {
	var rules []rule
	for service, spec := range specs {
		for path, item := range spec.Paths {
			ops := map[string]*huma.Operation{
				"GET": item.Get, "POST": item.Post, "PUT": item.Put, "PATCH": item.Patch,
				"DELETE": item.Delete, "HEAD": item.Head, "OPTIONS": item.Options,
			}
			for method, op := range ops {
				if op == nil {
					continue
				}
				access, ok := op.Extensions[httpx.AccessExtension].(string)
				if !ok {
					return nil, fmt.Errorf("%s %s (%s): register public operations with httpx.Register", method, path, service)
				}
				if !httpx.Access(access).Valid() {
					return nil, fmt.Errorf("%s %s (%s): unknown access %q", method, path, service, access)
				}
				r := rule{method: method, path: "/api" + pathParam.ReplaceAllString(path, "*"), access: access}
				var err error
				if r.timeout, err = duration(op, httpx.TimeoutExtension); err != nil {
					return nil, fmt.Errorf("%s %s: %w", method, path, err)
				}
				if r.cacheTTL, err = duration(op, httpx.CacheTTLExtension); err != nil {
					return nil, fmt.Errorf("%s %s: %w", method, path, err)
				}
				if v, ok := op.Extensions[httpx.MaxBodyExtension].(int64); ok {
					r.maxBody = v
				}
				rules = append(rules, r)
			}
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].path != rules[j].path {
			return rules[i].path < rules[j].path
		}
		return rules[i].method < rules[j].method
	})
	return rules, nil
}

func duration(op *huma.Operation, key string) (time.Duration, error) {
	raw, ok := op.Extensions[key].(string)
	if !ok {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d%time.Second != 0 {
		return 0, fmt.Errorf("%s: %s is not a whole number of seconds", key, raw)
	}
	return d, nil
}

func renderRules(rules []rule) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("package policy\n\nimport \"github.com/egor/k3s-base/internal/lib/httpx\"\n\nvar generated = Table{\n")
	for _, r := range rules {
		fmt.Fprintf(&b, "\t{Method: %q, Path: %q, Access: httpx.Access%s", r.method, r.path, strings.ToUpper(r.access[:1])+r.access[1:])
		if r.timeout > 0 {
			fmt.Fprintf(&b, ", Timeout: %s", durationExpr(r.timeout))
		}
		if r.maxBody > 0 {
			fmt.Fprintf(&b, ", MaxBody: %d", r.maxBody)
		}
		if r.cacheTTL > 0 {
			fmt.Fprintf(&b, ", CacheTTL: %s", durationExpr(r.cacheTTL))
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}

func durationExpr(d time.Duration) string {
	return fmt.Sprintf("sec(%d)", d/time.Second)
}

func mergeSpecs(specs map[string]*huma.OpenAPI) ([]byte, error) {
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)

	paths := map[string]any{}
	schemas := map[string]any{}
	security := map[string]any{}
	for _, name := range names {
		raw, err := specs[name].Downgrade()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		doc, err := parse(raw)
		if err != nil {
			return nil, err
		}
		components, _ := doc["components"].(map[string]any)
		own, _ := components["schemas"].(map[string]any)

		renames := map[string]string{}
		for schema, def := range own {
			if existing, taken := schemas[schema]; taken && !reflect.DeepEqual(existing, def) {
				renames[schema] = strings.ToUpper(name[:1]) + name[1:] + schema
			}
		}
		if len(renames) > 0 {
			text := string(raw)
			for old, renamed := range renames {
				text = strings.ReplaceAll(text, `"#/components/schemas/`+old+`"`, `"#/components/schemas/`+renamed+`"`)
			}
			if doc, err = parse([]byte(text)); err != nil {
				return nil, err
			}
			components, _ = doc["components"].(map[string]any)
			own, _ = components["schemas"].(map[string]any)
			for old, renamed := range renames {
				own[renamed] = own[old]
				delete(own, old)
			}
		}

		for path, item := range doc["paths"].(map[string]any) {
			if _, dup := paths[path]; dup {
				return nil, fmt.Errorf("path %s is declared by more than one service", path)
			}
			paths[path] = item
		}
		for schema, def := range own {
			schemas[schema] = def
		}
		if schemes, ok := components["securitySchemes"].(map[string]any); ok {
			for scheme, def := range schemes {
				security[scheme] = def
			}
		}
	}

	out := map[string]any{
		"openapi":    "3.0.3",
		"info":       map[string]any{"title": "API", "version": "v1"},
		"servers":    []any{map[string]any{"url": "/api"}},
		"paths":      paths,
		"components": map[string]any{"schemas": schemas, "securitySchemes": security},
	}
	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func parse(raw []byte) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}
