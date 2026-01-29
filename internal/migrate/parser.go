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
	Blocks     []Block
}

type Attribute struct {
	Name        string
	Type        string
	ElemType    string
	MinItems    *int
	MaxItems    *int
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

type Block struct {
	Name        string
	Kind        string
	Description string
	Attributes  []Attribute
}

type resolver struct {
	varMaps map[string]*ast.CompositeLit
	funcs   map[string]*ast.FuncDecl
}

func findProviderInfo(moduleRoot string) (ProviderInfo, error) {
	files, err := goFiles(moduleRoot)
	if err != nil {
		return ProviderInfo{}, err
	}

	fset := token.NewFileSet()
	parsed := make([]*ast.File, 0, len(files))
	paths := make([]string, 0, len(files))
	for _, file := range files {
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return ProviderInfo{}, err
		}
		parsed = append(parsed, node)
		paths = append(paths, file)
	}

	res := buildResolver(parsed)
	for i, node := range parsed {
		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Name.Name != "Provider" {
				continue
			}

			if !returnsSchemaProvider(fn.Type) {
				continue
			}

			info, ok, err := parseProviderFunction(fn, res)
			if err != nil {
				return ProviderInfo{}, fmt.Errorf("%s: %w", paths[i], err)
			}
			if ok {
				return info, nil
			}
		}
	}

	return ProviderInfo{}, fmt.Errorf("provider function not found")
}

func parseProviderFunction(fn *ast.FuncDecl, res resolver) (ProviderInfo, bool, error) {
	if fn.Body == nil {
		return ProviderInfo{}, false, nil
	}

	var providerLit *ast.CompositeLit
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.UnaryExpr:
			if expr.Op == token.AND {
				if lit, ok := expr.X.(*ast.CompositeLit); ok && isSchemaProviderType(lit.Type) {
					providerLit = lit
					return false
				}
			}
		case *ast.CompositeLit:
			if isSchemaProviderType(expr.Type) {
				providerLit = expr
				return false
			}
		}
		return true
	})

	if providerLit != nil {
		attrs, blocks, err := parseProviderComposite(providerLit, res)
		if err != nil {
			return ProviderInfo{}, false, err
		}
		return ProviderInfo{Attributes: attrs, Blocks: blocks}, true, nil
	}

	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}

		comp, ok := ret.Results[0].(*ast.UnaryExpr)
		if ok && comp.Op == token.AND {
			if lit, ok := comp.X.(*ast.CompositeLit); ok {
				attrs, blocks, err := parseProviderComposite(lit, res)
				if err != nil {
					return ProviderInfo{}, false, err
				}
				return ProviderInfo{Attributes: attrs, Blocks: blocks}, true, nil
			}
		}

		if ident, ok := ret.Results[0].(*ast.Ident); ok {
			if lit := findLocalProviderLiteral(fn, ident.Name); lit != nil {
				attrs, blocks, err := parseProviderComposite(lit, res)
				if err != nil {
					return ProviderInfo{}, false, err
				}
				return ProviderInfo{Attributes: attrs, Blocks: blocks}, true, nil
			}
		}
	}

	return ProviderInfo{}, false, nil
}

func parseProviderComposite(lit *ast.CompositeLit, res resolver) ([]Attribute, []Block, error) {
	if !isSchemaProviderType(lit.Type) {
		return nil, nil, fmt.Errorf("return value is not schema.Provider literal")
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

		attrs, blocks, err := parseSchemaMapExpr(kv.Value, res)
		if err != nil {
			return nil, nil, err
		}
		return attrs, blocks, nil
	}

	return nil, nil, nil
}

func parseSchemaMapExpr(expr ast.Expr, res resolver) ([]Attribute, []Block, error) {
	switch v := expr.(type) {
	case *ast.CompositeLit:
		return parseSchemaMap(v, res)
	case *ast.Ident:
		if lit, ok := res.varMaps[v.Name]; ok {
			return parseSchemaMap(lit, res)
		}
		return nil, nil, fmt.Errorf("schema map %q not resolved", v.Name)
	case *ast.CallExpr:
		fnName := functionName(v.Fun)
		if fnName == "" {
			return nil, nil, fmt.Errorf("schema map function not resolved")
		}
		if fn, ok := res.funcs[fnName]; ok {
			return parseSchemaMapFromFunc(fn, res)
		}
		return nil, nil, fmt.Errorf("schema map function %q not resolved", fnName)
	default:
		return nil, nil, fmt.Errorf("provider Schema is not a map literal")
	}
}

func parseSchemaMap(lit *ast.CompositeLit, res resolver) ([]Attribute, []Block, error) {
	if _, ok := lit.Type.(*ast.MapType); !ok && lit.Type != nil {
		return nil, nil, fmt.Errorf("provider Schema is not a map literal")
	}

	var attrs []Attribute
	var blocks []Block
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		name, ok := parseStringLiteral(kv.Key)
		if !ok {
			return nil, nil, fmt.Errorf("schema attribute name must be string literal")
		}

		attr, block, err := parseSchemaAttribute(name, kv.Value, res)
		if err != nil {
			return nil, nil, err
		}
		if block != nil {
			blocks = append(blocks, *block)
		} else {
			attrs = append(attrs, attr)
		}
	}

	return attrs, blocks, nil
}

func parseSchemaAttribute(name string, expr ast.Expr, res resolver) (Attribute, *Block, error) {
	lit, ok := expr.(*ast.UnaryExpr)
	if ok && lit.Op == token.AND {
		if comp, ok := lit.X.(*ast.CompositeLit); ok {
			return parseSchemaComposite(name, comp, res)
		}
	}

	if comp, ok := expr.(*ast.CompositeLit); ok {
		return parseSchemaComposite(name, comp, res)
	}

	return Attribute{}, nil, fmt.Errorf("schema attribute %q is not a schema.Schema literal", name)
}

func parseSchemaComposite(name string, lit *ast.CompositeLit, res resolver) (Attribute, *Block, error) {
	if lit.Type != nil && !isSchemaSchemaType(lit.Type) {
		return Attribute{}, nil, fmt.Errorf("schema attribute %q is not schema.Schema", name)
	}

	attr := Attribute{Name: name}
	var elemInfo elemInfo

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "Type":
			typ, err := parseSchemaType(kv.Value)
			if err != nil {
				return Attribute{}, nil, fmt.Errorf("schema attribute %q: %w", name, err)
			}
			attr.Type = typ
		case "Optional":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, nil, fmt.Errorf("schema attribute %q Optional must be bool literal", name)
			}
			attr.Optional = val
		case "Required":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, nil, fmt.Errorf("schema attribute %q Required must be bool literal", name)
			}
			attr.Required = val
		case "Computed":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, nil, fmt.Errorf("schema attribute %q Computed must be bool literal", name)
			}
			attr.Computed = val
		case "Sensitive":
			val, ok := parseBoolLiteral(kv.Value)
			if !ok {
				return Attribute{}, nil, fmt.Errorf("schema attribute %q Sensitive must be bool literal", name)
			}
			attr.Sensitive = val
		case "Description":
			if val, ok := parseStringExpr(kv.Value); ok {
				attr.Description = val
			}
		case "Elem":
			info, err := parseElem(kv.Value, res)
			if err != nil {
				return Attribute{}, nil, fmt.Errorf("schema attribute %q: %w", name, err)
			}
			elemInfo = info
		case "MinItems":
			if val, ok := parseIntLiteral(kv.Value); ok {
				attr.MinItems = &val
			}
		case "MaxItems":
			if val, ok := parseIntLiteral(kv.Value); ok {
				attr.MaxItems = &val
			}
		}
	}

	if attr.Type == "" {
		return Attribute{}, nil, fmt.Errorf("schema attribute %q missing Type", name)
	}

	if elemInfo.isResource {
		if attr.Type != "list" && attr.Type != "set" {
			return Attribute{}, nil, fmt.Errorf("schema attribute %q has resource Elem but type is %s", name, attr.Type)
		}

		if len(elemInfo.blocks) > 0 {
			return Attribute{}, nil, fmt.Errorf("schema attribute %q has nested blocks inside Elem resource (unsupported)", name)
		}

		block := Block{
			Name:        name,
			Kind:        attr.Type,
			Description: attr.Description,
			Attributes:  elemInfo.attrs,
		}
		return Attribute{}, &block, nil
	}

	if elemInfo.elemType != "" {
		attr.ElemType = elemInfo.elemType
	}

	if (attr.Type == "list" || attr.Type == "set" || attr.Type == "map") && attr.ElemType == "" {
		return Attribute{}, nil, fmt.Errorf("schema attribute %q missing Elem type", name)
	}

	return attr, nil, nil
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

type elemInfo struct {
	elemType   string
	attrs      []Attribute
	blocks     []Block
	isResource bool
}

func parseElem(expr ast.Expr, res resolver) (elemInfo, error) {
	lit, ok := expr.(*ast.UnaryExpr)
	if ok && lit.Op == token.AND {
		if comp, ok := lit.X.(*ast.CompositeLit); ok {
			return parseElemFromComposite(comp, res)
		}
	}

	if comp, ok := expr.(*ast.CompositeLit); ok {
		return parseElemFromComposite(comp, res)
	}

	return elemInfo{}, fmt.Errorf("Elem must be schema.Schema or schema.Resource literal")
}

func parseElemFromComposite(lit *ast.CompositeLit, res resolver) (elemInfo, error) {
	if lit.Type == nil || isSchemaSchemaType(lit.Type) {
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Type" {
				continue
			}

			elemType, err := parseSchemaType(kv.Value)
			if err != nil {
				return elemInfo{}, err
			}
			return elemInfo{elemType: elemType}, nil
		}
		return elemInfo{}, fmt.Errorf("Elem schema missing Type")
	}

	if isSchemaResourceType(lit.Type) {
		attrs, blocks, err := parseResourceSchema(lit, res)
		if err != nil {
			return elemInfo{}, err
		}
		return elemInfo{attrs: attrs, blocks: blocks, isResource: true}, nil
	}

	return elemInfo{}, fmt.Errorf("Elem must be schema.Schema or schema.Resource literal")
}

func parseResourceSchema(lit *ast.CompositeLit, res resolver) ([]Attribute, []Block, error) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Schema" {
			continue
		}
		return parseSchemaMapExpr(kv.Value, res)
	}

	return nil, nil, fmt.Errorf("resource Schema field not found")
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

func isSchemaResourceType(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.StarExpr:
		return isSchemaResourceType(v.X)
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok && ident.Name == "schema" && v.Sel.Name == "Resource" {
			return true
		}
	case *ast.Ident:
		return v.Name == "Resource"
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

func parseStringExpr(expr ast.Expr) (string, bool) {
	if val, ok := parseStringLiteral(expr); ok {
		return val, true
	}
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		left, ok := parseStringExpr(bin.X)
		if !ok {
			return "", false
		}
		right, ok := parseStringExpr(bin.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	}
	return "", false
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

func parseIntLiteral(expr ast.Expr) (int, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || (lit.Kind != token.INT && lit.Kind != token.FLOAT) {
		return 0, false
	}
	val, err := strconv.Atoi(strings.TrimSpace(lit.Value))
	if err != nil {
		return 0, false
	}
	return val, true
}

func buildResolver(files []*ast.File) resolver {
	res := resolver{
		varMaps: map[string]*ast.CompositeLit{},
		funcs:   map[string]*ast.FuncDecl{},
	}

	for _, node := range files {
		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name != nil && d.Name.Name != "" {
					res.funcs[d.Name.Name] = d
				}
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range valueSpec.Names {
						if i >= len(valueSpec.Values) {
							continue
						}
						if lit, ok := valueSpec.Values[i].(*ast.CompositeLit); ok {
							if _, ok := lit.Type.(*ast.MapType); ok || lit.Type == nil {
								res.varMaps[name.Name] = lit
							}
						}
					}
				}
			}
		}
	}

	return res
}

func parseSchemaMapFromFunc(fn *ast.FuncDecl, res resolver) ([]Attribute, []Block, error) {
	if fn.Body == nil {
		return nil, nil, fmt.Errorf("schema map function has no body")
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		if ident, ok := ret.Results[0].(*ast.Ident); ok {
			if lit := findLocalMapLiteral(fn, ident.Name); lit != nil {
				return parseSchemaMap(lit, res)
			}
		}
		return parseSchemaMapExpr(ret.Results[0], res)
	}
	return nil, nil, fmt.Errorf("schema map function has no return")
}

func functionName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	default:
		return ""
	}
}

func findLocalMapLiteral(fn *ast.FuncDecl, name string) *ast.CompositeLit {
	if fn.Body == nil {
		return nil
	}

	for _, stmt := range fn.Body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name != name || i >= len(s.Rhs) {
					continue
				}
				if lit, ok := s.Rhs[i].(*ast.CompositeLit); ok {
					return lit
				}
			}
		case *ast.DeclStmt:
			gen, ok := s.Decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range valueSpec.Names {
					if ident.Name != name || i >= len(valueSpec.Values) {
						continue
					}
					if lit, ok := valueSpec.Values[i].(*ast.CompositeLit); ok {
						return lit
					}
				}
			}
		}
	}

	return nil
}

func findLocalProviderLiteral(fn *ast.FuncDecl, name string) *ast.CompositeLit {
	if fn.Body == nil {
		return nil
	}

	for _, stmt := range fn.Body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name != name || i >= len(s.Rhs) {
					continue
				}
				if lit, ok := s.Rhs[i].(*ast.CompositeLit); ok {
					return lit
				}
				if unary, ok := s.Rhs[i].(*ast.UnaryExpr); ok && unary.Op == token.AND {
					if lit, ok := unary.X.(*ast.CompositeLit); ok {
						return lit
					}
				}
			}
		case *ast.DeclStmt:
			gen, ok := s.Decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range valueSpec.Names {
					if ident.Name != name || i >= len(valueSpec.Values) {
						continue
					}
					if lit, ok := valueSpec.Values[i].(*ast.CompositeLit); ok {
						return lit
					}
					if unary, ok := valueSpec.Values[i].(*ast.UnaryExpr); ok && unary.Op == token.AND {
						if lit, ok := unary.X.(*ast.CompositeLit); ok {
							return lit
						}
					}
				}
			}
		}
	}

	return nil
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
