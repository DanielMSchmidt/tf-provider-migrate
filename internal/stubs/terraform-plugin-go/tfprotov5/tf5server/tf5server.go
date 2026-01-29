package tf5server

import "github.com/hashicorp/terraform-plugin-go/tfprotov5"

type ServeOpt interface{}

func Serve(_ string, _ func() tfprotov5.ProviderServer, _ ...ServeOpt) error {
	return nil
}
