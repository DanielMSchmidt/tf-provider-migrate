package migrate

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func findMainInfo(moduleRoot string) (string, MainInfo, error) {
	var mainFile string
	var info MainInfo

	files, err := goFiles(moduleRoot)
	if err != nil {
		return "", MainInfo{}, err
	}

	for _, file := range files {
		if filepath.Base(file) != "main.go" {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return "", MainInfo{}, err
		}

		if node.Name == nil || node.Name.Name != "main" {
			continue
		}

		mainFile = file
		info = parseMainFile(node)
		info.BuildTags, info.GoGenerate = extractMainDirectives(file)
		break
	}

	if mainFile == "" {
		return "", MainInfo{}, fmt.Errorf("main.go not found")
	}

	return mainFile, info, nil
}

func parseMainFile(node *ast.File) MainInfo {
	info := MainInfo{}

	providerAlias := ""
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "main" || fn.Body == nil {
			continue
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "Provider" {
				return true
			}

			if ident, ok := sel.X.(*ast.Ident); ok {
				providerAlias = ident.Name
				return false
			}
			return true
		})
	}

	if providerAlias != "" {
		info.ProviderAlias = providerAlias
		info.ProviderImport = importPathForAlias(node, providerAlias)
	}

	return info
}

func importPathForAlias(node *ast.File, alias string) string {
	for _, imp := range node.Imports {
		path, err := strconvUnquote(imp.Path.Value)
		if err != nil {
			continue
		}

		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			name = parts[len(parts)-1]
		}

		if name == alias {
			return path
		}
	}
	return ""
}

func extractMainDirectives(path string) ([]string, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer file.Close()

	var buildTags []string
	var goGenerate []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "package ") {
			break
		}

		if strings.HasPrefix(trim, "//go:build") || strings.HasPrefix(trim, "// +build") {
			buildTags = append(buildTags, trim)
			continue
		}

		if strings.HasPrefix(trim, "//go:generate") {
			goGenerate = append(goGenerate, trim)
		}
	}

	return buildTags, goGenerate
}

func strconvUnquote(raw string) (string, error) {
	if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '`') {
		return strconv.Unquote(raw)
	}
	return raw, nil
}
