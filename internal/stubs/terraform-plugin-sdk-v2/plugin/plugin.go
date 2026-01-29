package plugin

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ServeOpts struct {
	ProviderFunc func() *schema.Provider
}

func Serve(_ *ServeOpts) {}
