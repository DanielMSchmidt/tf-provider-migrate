package providerserver

import (
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

func NewProtocol5(_ provider.Provider) func() tfprotov5.ProviderServer {
	return func() tfprotov5.ProviderServer { return nil }
}
