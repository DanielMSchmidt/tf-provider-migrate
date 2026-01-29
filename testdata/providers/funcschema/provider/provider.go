package provider

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

func Provider() *schema.Provider {
	p := &schema.Provider{
		Schema: providerSchema(),
	}
	return p
}
