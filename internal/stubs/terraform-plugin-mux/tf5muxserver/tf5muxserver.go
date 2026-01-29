package tf5muxserver

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

type MuxServer struct{}

func NewMuxServer(_ context.Context, _ ...func() tfprotov5.ProviderServer) (*MuxServer, error) {
	return &MuxServer{}, nil
}

func (m *MuxServer) ProviderServer() tfprotov5.ProviderServer {
	return nil
}
