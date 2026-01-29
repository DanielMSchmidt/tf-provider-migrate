package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

var providerSchema = map[string]*schema.Schema{
	"api_url": {
		Type:        schema.TypeString,
		Optional:    true,
		DefaultFunc: schema.EnvDefaultFunc("VARS_API_URL", "https://example.com"),
		Description: "Base URL for the API",
	},
	"auth": {
		Type:        schema.TypeList,
		Optional:    true,
		MinItems:    1,
		MaxItems:    1,
		Description: "Authentication settings",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"token": {
					Type:        schema.TypeString,
					Required:    true,
					Sensitive:   true,
					Description: "API token",
				},
				"profile": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Optional profile name",
				},
			},
		},
	},
}

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: providerSchema,
	}
}
