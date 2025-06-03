package component_tests_test

import (
	"context"
	"log"
	"net"

	"code.cloudfoundry.org/tlsconfig"
	"code.cloudfoundry.org/tlsconfig/certtest"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"github.com/cloudfoundry/statsd-injector/component_tests/component_testsfakes"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// grpcMetronAdapter adapts our counterfeiter mock to the gRPC interface
type grpcMetronAdapter struct {
	loggregator_v2.UnimplementedIngressServer
	mock *component_testsfakes.FakeMetronIngressServer
}

func (a *grpcMetronAdapter) Sender(stream loggregator_v2.Ingress_SenderServer) error {
	return a.mock.Sender(stream)
}

func (a *grpcMetronAdapter) BatchSender(stream loggregator_v2.Ingress_BatchSenderServer) error {
	return a.mock.BatchSender(stream)
}

func (a *grpcMetronAdapter) Send(ctx context.Context, batch *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	return a.mock.Send(ctx, batch)
}

type MetronServer struct {
	port     int
	server   *grpc.Server
	listener net.Listener
	Metron   *component_testsfakes.FakeMetronIngressServer
}

func NewMetronServer(ca *certtest.Authority, caFile string) (*MetronServer, error) {
	certPath, keyPath := GenerateCertKey("metron", ca)
	config, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certPath, keyPath),
	).Client(
		tlsconfig.WithAuthorityFromFile(caFile),
		tlsconfig.WithServerName("metron"),
	)
	if err != nil {
		log.Fatal("Invalid TLS credentials")
	}

	transportCreds := credentials.NewTLS(config)
	mockMetron := &component_testsfakes.FakeMetronIngressServer{}

	// Configure the mock to simulate receiving envelopes
	mockMetron.SenderStub = func(stream loggregator_v2.Ingress_SenderServer) error {
		// Keep receiving until the context is done
		for {
			_, err := stream.Recv()
			if err != nil {
				return err
			}
		}
	}

	adapter := &grpcMetronAdapter{mock: mockMetron}

	lis, err := net.Listen("tcp", ":0") //nolint:gosec
	if err != nil {
		return nil, err
	}
	s := grpc.NewServer(grpc.Creds(transportCreds))
	loggregator_v2.RegisterIngressServer(s, adapter)

	go s.Serve(lis) //nolint:errcheck

	return &MetronServer{
		port:     lis.Addr().(*net.TCPAddr).Port,
		server:   s,
		listener: lis,
		Metron:   mockMetron,
	}, nil
}

func (s *MetronServer) URI() string {
	return s.listener.Addr().String()
}

func (s *MetronServer) Port() int {
	return s.port
}

func (s *MetronServer) Stop() error {
	err := s.listener.Close()
	s.server.Stop()
	return err
}
