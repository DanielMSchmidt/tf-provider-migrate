package migrate

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"sort"
	"strings"
	"text/template"
)

func renderFrameworkProvider(info ProviderInfo, providerName string) ([]byte, error) {
	attrs := make([]Attribute, len(info.Attributes))
	copy(attrs, info.Attributes)
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Name < attrs[j].Name
	})

	blocks := make([]Block, len(info.Blocks))
	copy(blocks, info.Blocks)
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Name < blocks[j].Name
	})

	useTypes := false
	for _, attr := range attrs {
		if attr.Type == "list" || attr.Type == "set" || attr.Type == "map" {
			useTypes = true
			break
		}
	}
	if !useTypes {
		for _, block := range blocks {
			for _, attr := range block.Attributes {
				if attr.Type == "list" || attr.Type == "set" || attr.Type == "map" {
					useTypes = true
					break
				}
			}
			if useTypes {
				break
			}
		}
	}

	data := map[string]interface{}{
		"ProviderName": providerName,
		"Attributes":   attrs,
		"Blocks":       blocks,
		"UseTypes":     useTypes,
	}

	var buf bytes.Buffer
	tmpl := template.Must(template.New("framework").Funcs(template.FuncMap{
		"attrLiteral":  renderAttributeLiteral,
		"blockLiteral": renderBlockLiteral,
		"elementType":  renderElementType,
	}).Parse(frameworkTemplate))

	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

func renderMuxedMain(info MainInfo, registryAddress string) ([]byte, error) {
	data := map[string]interface{}{
		"BuildTags":       info.BuildTags,
		"GoGenerate":      info.GoGenerate,
		"ProviderImport":  info.ProviderImport,
		"ProviderAlias":   info.ProviderAlias,
		"FrameworkImport": deriveFrameworkImport(info.ProviderImport),
		"Registry":        registryAddress,
	}

	var buf bytes.Buffer
	tmpl := template.Must(template.New("main").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(mainTemplate))

	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

func renderAttributeLiteral(attr Attribute) string {
	attrType := frameworkAttributeType(attr.Type)
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "schema.%s{", attrType)
	if attr.Description != "" {
		fmt.Fprintf(&buf, "Description: %q,", attr.Description)
	}
	if attr.Required {
		fmt.Fprintf(&buf, "Required: true,")
	}
	if attr.Optional {
		fmt.Fprintf(&buf, "Optional: true,")
	}
	if attr.Computed {
		fmt.Fprintf(&buf, "Computed: true,")
	}
	if attr.Sensitive {
		fmt.Fprintf(&buf, "Sensitive: true,")
	}
	if attr.Type == "list" || attr.Type == "set" || attr.Type == "map" {
		fmt.Fprintf(&buf, "ElementType: %s,", renderElementType(attr))
	}
	buf.WriteString("}")
	return buf.String()
}

func renderBlockLiteral(block Block) string {
	var buf bytes.Buffer

	blockType := "ListNestedBlock"
	if block.Kind == "set" {
		blockType = "SetNestedBlock"
	}

	fmt.Fprintf(&buf, "schema.%s{", blockType)
	if block.Description != "" {
		fmt.Fprintf(&buf, "Description: %q,", block.Description)
	}
	buf.WriteString("NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{")

	attrs := make([]Attribute, len(block.Attributes))
	copy(attrs, block.Attributes)
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Name < attrs[j].Name
	})

	for _, attr := range attrs {
		fmt.Fprintf(&buf, "%q: %s,", attr.Name, renderAttributeLiteral(attr))
	}
	buf.WriteString("}},}")

	return buf.String()
}

func renderElementType(attr Attribute) string {
	switch attr.ElemType {
	case "string":
		return "types.StringType"
	case "bool":
		return "types.BoolType"
	case "int":
		return "types.Int64Type"
	case "float":
		return "types.Float64Type"
	default:
		return "types.StringType"
	}
}

func frameworkAttributeType(typ string) string {
	switch typ {
	case "string":
		return "StringAttribute"
	case "bool":
		return "BoolAttribute"
	case "int":
		return "Int64Attribute"
	case "float":
		return "Float64Attribute"
	case "list":
		return "ListAttribute"
	case "set":
		return "SetAttribute"
	case "map":
		return "MapAttribute"
	default:
		return "StringAttribute"
	}
}

func deriveFrameworkImport(providerImport string) string {
	if providerImport == "" {
		return ""
	}
	return path.Join(path.Dir(providerImport), "framework")
}

const frameworkTemplate = `package framework

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	{{- if .UseTypes }}
	"github.com/hashicorp/terraform-plugin-framework/types"
	{{- end }}
)

var _ provider.Provider = (*fwprovider)(nil)

type fwprovider struct {
	Primary interface {
		Meta() interface{}
	}
}

func New(primary interface{ Meta() interface{} }) provider.Provider {
	return &fwprovider{Primary: primary}
}

func (p *fwprovider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "{{ .ProviderName }}"
}

func (p *fwprovider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			{{- range .Attributes }}
			"{{ .Name }}": {{ attrLiteral . }},
			{{- end }}
		},
		Blocks: map[string]schema.Block{
			{{- range .Blocks }}
			"{{ .Name }}": {{ blockLiteral . }},
			{{- end }}
		},
	}
}

func (p *fwprovider) Configure(_ context.Context, _ provider.ConfigureRequest, response *provider.ConfigureResponse) {
	response.DataSourceData = p.Primary.Meta()
	response.ResourceData = p.Primary.Meta()
}

func (p *fwprovider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func (p *fwprovider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}
`

const mainTemplate = `{{- if .BuildTags }}{{ join .BuildTags "\n" }}{{ "\n\n" }}{{- end -}}
package main

import (
	"context"
	"log"

	"{{ .FrameworkImport }}"
	"{{ .ProviderImport }}"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

{{- if .GoGenerate }}
{{ join .GoGenerate "\n" }}

{{- end }}
func main() {
	primary := {{ .ProviderAlias }}.Provider()

	ctx := context.Background()
	muxServer, err := tf5muxserver.NewMuxServer(ctx,
		func() tfprotov5.ProviderServer {
			return schema.NewGRPCProviderServer(primary)
		},
		providerserver.NewProtocol5(framework.New(primary)),
	)
	if err != nil {
		log.Fatal(err)
	}

	goServeOpts := []tf5server.ServeOpt{}
	err = tf5server.Serve(
		"{{ .Registry }}",
		muxServer.ProviderServer,
		goServeOpts...,
	)
	if err != nil {
		log.Fatal(err)
	}
}
`
