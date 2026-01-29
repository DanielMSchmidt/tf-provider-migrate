package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"region": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Region to operate in",
			},
			"project": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Project override",
			},
			"debug": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Enable debug logging",
			},
			"retry_count": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Retry count",
			},
			"retry_backoff": &schema.Schema{
				Type:        schema.TypeFloat,
				Optional:    true,
				Description: "Retry backoff",
			},
			"endpoints": &schema.Schema{
				Type:        schema.TypeList,
				Optional:    true,
				Description: "API endpoints",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"tags": &schema.Schema{
				Type:        schema.TypeMap,
				Optional:    true,
				Description: "Default tags",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		ResourcesMap:   map[string]*schema.Resource{},
		DataSourcesMap: map[string]*schema.Resource{},
	}
}
