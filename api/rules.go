package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"sort"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

type rule struct {
	method, path, access string
}

func writeGatewayRules(specs map[string]*huma.OpenAPI) error {
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
					return fmt.Errorf("%s %s (%s): register public operations with httpx.Register", method, path, service)
				}
				rules = append(rules, rule{method, "/api" + pathParam.ReplaceAllString(path, "*"), access})
			}
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].path != rules[j].path {
			return rules[i].path < rules[j].path
		}
		return rules[i].method < rules[j].method
	})

	var b bytes.Buffer
	b.WriteString("package gateway\n\nvar generatedRules = []Rule{\n")
	for _, r := range rules {
		fmt.Fprintf(&b, "\t{Method: %q, Path: %q, Access: %s},\n", r.method, r.path, accessConst(r.access))
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile("internal/gateway/rules_gen.go", src, 0o600)
}

func accessConst(access string) string {
	switch httpx.Access(access) {
	case httpx.AccessPublic:
		return "AccessPublic"
	case httpx.AccessAdmin:
		return "AccessAdmin"
	default:
		return "AccessUser"
	}
}
