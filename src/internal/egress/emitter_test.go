package egress_test

import (
	"context"
	"errors"
	"log"
	"net"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"github.com/cloudfoundry/statsd-injector/internal/egress"
	"github.com/cloudfoundry/statsd-injector/internal/egress/egressfakes"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// grpcMetronAdapter adapts our counterfeiter mock to the gRPC interface
type grpcMetronAdapter struct {
	loggregator_v2.UnimplementedIngressServer
	mock *egressfakes.FakeMetronIngressServer
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

var errStreamClosed = errors.New("stream closed")
var _ = Describe("Statsdemitter", func() {
	var (
		serverAddr string
		mockServer *egressfakes.FakeMetronIngressServer
		inputChan  chan *loggregator_v2.Envelope
		message    *loggregator_v2.Envelope
	)

	BeforeEach(func() {
		inputChan = make(chan *loggregator_v2.Envelope, 100)
		message = &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{
					Name:  "a-name",
					Delta: 48,
				},
			},
		}
	})

	Context("when the server is already listening", func() {
		BeforeEach(func() {
			serverAddr, mockServer = startServer()

			// Configure the mock to simulate receiving envelopes
			mockServer.SenderStub = func(stream loggregator_v2.Ingress_SenderServer) error {
				// Keep receiving until the context is done
				for {
					_, err := stream.Recv()
					if err != nil {
						return err
					}
				}
			}

			dialOpt := grpc.WithTransportCredentials(insecure.NewCredentials())
			emitter := egress.New(serverAddr, dialOpt)

			go emitter.Run(inputChan)
		})

		It("emits envelope", func() {
			go keepWriting(inputChan, message)
			Eventually(func() int {
				return mockServer.SenderCallCount()
			}).Should(BeNumerically(">", 0))
		})

		It("reconnects when the stream has been closed", func() {
			go keepWriting(inputChan, message)
			// Set up the mock to return an error to simulate stream closure
			mockServer.SenderReturns(errStreamClosed)

			Eventually(func() int {
				return mockServer.SenderCallCount()
			}).Should(BeNumerically(">", 1))
		})
	})
})

func keepWriting(c chan<- *loggregator_v2.Envelope, e *loggregator_v2.Envelope) {
	for {
		c <- e
	}
}

func startServer() (string, *egressfakes.FakeMetronIngressServer) {
	lis, err := net.Listen("tcp", ":0") //nolint:gosec
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer()
	mockMetronIngressServer := &egressfakes.FakeMetronIngressServer{}
	adapter := &grpcMetronAdapter{mock: mockMetronIngressServer}
	loggregator_v2.RegisterIngressServer(s, adapter)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Printf("Failed to start server: %s", err)
		}
	}()

	return lis.Addr().String(), mockMetronIngressServer
}
