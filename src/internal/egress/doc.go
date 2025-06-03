package egress

import (
	"context"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . MetronIngressServer

// MetronIngressServer wraps the gRPC interface methods we need to mock
type MetronIngressServer interface {
	Sender(loggregator_v2.Ingress_SenderServer) error
	BatchSender(loggregator_v2.Ingress_BatchSenderServer) error
	Send(context.Context, *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error)
}
