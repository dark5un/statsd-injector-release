package component_tests

import "code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . MetronIngressServer
type MetronIngressServer interface {
	loggregator_v2.IngressServer
}
