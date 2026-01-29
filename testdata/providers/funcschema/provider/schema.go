package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func providerSchema() map[string]*schema.Schema {
	schemaMap := map[string]*schema.Schema{
		"endpoint": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Endpoint for API access",
		},
		"debug": {
			Type:        schema.TypeBool,
			Optional:    true,
			Description: "Enable debug logging",
		},
	}
	return schemaMap
}
