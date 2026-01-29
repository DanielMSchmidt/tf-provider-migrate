package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type Provider interface {
	Metadata(context.Context, MetadataRequest, *MetadataResponse)
	Schema(context.Context, SchemaRequest, *SchemaResponse)
	Configure(context.Context, ConfigureRequest, *ConfigureResponse)
	DataSources(context.Context) []func() datasource.DataSource
	Resources(context.Context) []func() resource.Resource
}

type MetadataRequest struct{}
type MetadataResponse struct {
	TypeName string
}

type SchemaRequest struct{}
type SchemaResponse struct {
	Schema schema.Schema
}

type ConfigureRequest struct{}
type ConfigureResponse struct {
	DataSourceData interface{}
	ResourceData   interface{}
}
