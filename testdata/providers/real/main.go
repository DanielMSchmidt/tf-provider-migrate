//go:generate go test ./...
package main

import (
	"github.com/examplecorp/terraform-provider-realistic/provider"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: provider.Provider,
	})
}
