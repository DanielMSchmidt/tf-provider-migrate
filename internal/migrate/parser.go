package migrate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

type ProviderInfo struct {
	Attributes []Attribute
}

type Attribute struct {
	Name        string
	Type        string
	ElemType    string
	Optional    bool
	Required    bool
	Computed    bool
	Sensitive   bool
	Description string
}

type MainInfo struct {
	ProviderImport string
	ProviderAlias  string
	BuildTags      []string
	GoGenerate     []string
}

func findProviderInfo(moduleRoot string) (ProviderInfo, error) {
	files, err := goFiles(moduleRoot)
	if err != nil {
		return ProviderInfo{}, err
	}

	for _, file := range files {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return ProviderInfo{}, err
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Name.Name != "Provider" {
				continue
			}

			if !returnsSchemaProvider(fn.Type) {
				continue
			}

			info, ok, err := parseProviderFunction(fn)
			if err != nil {
				return ProviderInfo{}, fmt.Errorf("%s: %w", file, err)
			}
			if ok {
				return info, nil
			}
		}
	}

	return ProviderInfo{}, fmt.Errorf("provider function not found")
}

func parseProviderFunction(fn *ast.FuncDecl) (ProviderInfo, bool, error) {
	if fn.Body == nil {
		return ProviderInfo{}, false, nil
	}

	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}

		comp, ok := ret.Results[0].(*ast.UnaryExpr)
		if ok && comp.Op == token.AND {
			if lit, ok := comp.X.(*ast.CompositeLit); ok {
				attrs, err := parseProviderComposite(lit)
				if err != nil {
					return ProviderInfo{}, false, err
				}
				return ProviderInfo{Attributes: attrs}, true, nil
			}
		}
	}

	return ProviderInfo{}, false, nil
}

func parseProviderComposite(lit *ast.CompositeLit) ([]Attribute, error) {
	if !isSchemaProviderType(lit.Type) {
		return nil, fmt.Errorf("return value is not schema.Provider literal")
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Schema" {
			continue
		}

		mapLit, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("provider Schema is not a composite literal")
		}

		return parseSchemaMap(mapLit)
	}

	return nil, fmt.Errorf("provider Schema field not found")
}

func parseSchemaMap(lit *ast.CompositeLit) ([]Attribute, error) {
	if _, ok := lit.Type.(*ast.MapType); !ok && lit.Type != nil {
		return nil, fmt.Errorf("provider Schema is not a map literal")
	}

	var attrs []Attribute
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		name, ok := parseStringLiteral(kv.Key)
		if !ok {
			return nil, fmt.Errorf("schema attribute name must be string literal")
		}

		attr, err := parseSchemaAttribute(name, kv.Value)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, attr)
	}

	return attrs, nil
}

func parseSchemaAttribute(name string, expr ast.Expr) (Attribute, error) {
	lit, ok := expr.(*ast.UnaryExpr)
	if ok && lit.Op == token.AND {
		if comp, ok := lit.X.(*ast.CompositeLit); ok {
			return parseSchemaComposite(name, comp)
		}
	}

	if comp, ok := expr.(*ast.CompositeLit); ok {
		return parseSchemaComposite(name, comp)
	}

	return Attribute{}, fmt.Errorf("schema attribute %q is not a schema.Schema literal", name)
}

func parseSchemaComposite(name string, lit *ast.CompositeLit) (Attribute, error) {
	if !isSchemaSchemaType(lit.Type) {
		return Attribute{}, fmt.Errorf("schema attribute %q is not schema.Schema", name)
	}

	attr := Attribute{Name: name}
	allowedFields := map[string]bool{
		"Type":        true,
		"Optional":    true,
		"Required":    true,
		"Computed":    true,
		"Sensitive":   true,
		"Description": true,
		"Elem":        true,
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		if !allowedFields[key.Name] {
			return Attribute{}, fmt.Errorf("schema attribute %q uses unsupported field %q", name, key.Name)
		}

		switch key.Name {
		case "Type":
			typ, err := parseSchemaType(kv.Value)
			if err != nil {
				return Attribute{}, fmt.Errorf("schema attribute %q: %w", name, err)
			}
			attr.Type = typ
		case "Optional":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, fmt.Errorf("schema attribute %q Optional must be bool literal", name)
			}
			attr.Optional = val
		case "Required":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, fmt.Errorf("schema attribute %q Required must be bool literal", name)
			}
			attr.Required = val
		case "Computed":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, fmt.Errorf("schema attribute %q Computed must be bool literal", name)
			}
			attr.Computed = val
		case "Sensitive":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, fmt.Errorf("schema attribute %q Sensitive must be bool literal", name)
			}
			attr.Sensitive = val
		case "Description":
			val, ok := parseStringLiteral(kv.Value)
			if !ok {
				return Attribute{}, fmt.Errorf("schema attribute %q Description must be string literal", name)
			}
			attr.Description = val
		case "Elem":
			elemType, err := parseElemType(kv.Value)
			if err != nil {
				return Attribute{}, fmt.Errorf("schema attribute %q: %w", name, err)
			}
			attr.ElemType = elemType
		}
	}

	if attr.Type == "" {
		return Attribute{}, fmt.Errorf("schema attribute %q missing Type", name)
	}

	if (attr.Type == "list" || attr.Type == "set" || attr.Type == "map") && attr.ElemType == "" {
		return Attribute{}, fmt.Errorf("schema attribute %q missing Elem type", name)
	}

	return attr, nil
}

func parseSchemaType(expr ast.Expr) (string, error) {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok && ident.Name == "schema" {
			return normalizeSchemaType(v.Sel.Name)
		}
	case *ast.Ident:
		return normalizeSchemaType(v.Name)
	}

	return "", fmt.Errorf("unsupported schema type")
}

func normalizeSchemaType(name string) (string, error) {
	switch name {
	case "TypeString":
		return "string", nil
	case "TypeBool":
		return "bool", nil
	case "TypeInt":
		return "int", nil
	case "TypeFloat":
		return "float", nil
	case "TypeList":
		return "list", nil
	case "TypeSet":
		return "set", nil
	case "TypeMap":
		return "map", nil
	default:
		return "", fmt.Errorf("unsupported schema type %s", name)
	}
}

func parseElemType(expr ast.Expr) (string, error) {
	lit, ok := expr.(*ast.UnaryExpr)
	if ok && lit.Op == token.AND {
		if comp, ok := lit.X.(*ast.CompositeLit); ok {
			return parseElemTypeFromComposite(comp)
		}
	}

	if comp, ok := expr.(*ast.CompositeLit); ok {
		return parseElemTypeFromComposite(comp)
	}

	return "", fmt.Errorf("Elem must be schema.Schema literal")
}

func parseElemTypeFromComposite(lit *ast.CompositeLit) (string, error) {
	if !isSchemaSchemaType(lit.Type) {
		return "", fmt.Errorf("Elem must be schema.Schema literal")
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Type" {
			continue
		}

		return parseSchemaType(kv.Value)
	}

	return "", fmt.Errorf("Elem schema missing Type")
}

func returnsSchemaProvider(fnType *ast.FuncType) bool {
	if fnType.Results == nil || len(fnType.Results.List) == 0 {
		return false
	}

	for _, result := range fnType.Results.List {
		if isSchemaProviderType(result.Type) {
			return true
		}
	}
	return false
}

func isSchemaProviderType(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.StarExpr:
		return isSchemaProviderType(v.X)
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok && ident.Name == "schema" && v.Sel.Name == "Provider" {
			return true
		}
	case *ast.Ident:
		return v.Name == "Provider"
	}
	return false
}

func isSchemaSchemaType(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.StarExpr:
		return isSchemaSchemaType(v.X)
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok && ident.Name == "schema" && v.Sel.Name == "Schema" {
			return true
		}
	case *ast.Ident:
		return v.Name == "Schema"
	}
	return false
}

func parseStringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func parseBoolLiteral(expr ast.Expr) (bool, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false, false
	}
	if ident.Name == "true" {
		return true, true
	}
	if ident.Name == "false" {
		return false, true
	}
	return false, false
}

func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "vendor", "testdata", ".beads", ".repo.git", ".github":
		return true
	default:
		return false
	}
}
