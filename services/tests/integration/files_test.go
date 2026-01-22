package integration

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	consequtiveFailures = 3
	timeout             = 1 * time.Second
)

func startTestServer(t *testing.T, srv *testFilesServer) (addr string, cleanup func()) {

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterUploaderServer(grpcServer, srv)
	go grpcServer.Serve(lis)
	return lis.Addr().String(), func() {
		grpcServer.Stop()
		lis.Close()
	}
}

func newBreaker() *gobreaker.CircuitBreaker[*pb.FilesReply] {
	return gobreaker.NewCircuitBreaker[*pb.FilesReply](gobreaker.Settings{
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= consequtiveFailures
		},
		Timeout: timeout,
	})
}

func TestBreaker_Success(t *testing.T) {
	srv := &testFilesServer{}
	srv.mode.Store(0) // always success

	addr, cleanup := startTestServer(t, srv)
	defer cleanup()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUploaderClient(conn)
	breaker := newBreaker()

	for i := 0; i < consequtiveFailures; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, err := breaker.Execute(func() (*pb.FilesReply, error) {
			return client.GetFiles(ctx, &pb.UserInfo{})
		})
		cancel()
		assert.NoError(t, err)
	}

	assert.Equal(t, gobreaker.StateClosed, breaker.State())
}

func TestBreaker_TripsWithGrpcFailures(t *testing.T) {
	srv := &testFilesServer{}
	srv.mode.Store(1) // always error

	addr, cleanup := startTestServer(t, srv)
	defer cleanup()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUploaderClient(conn)
	breaker := newBreaker()

	for i := 0; i < consequtiveFailures; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, _ = breaker.Execute(func() (*pb.FilesReply, error) {
			return client.GetFiles(ctx, &pb.UserInfo{})
		})
		cancel()
	}

	assert.Equal(t, gobreaker.StateOpen, breaker.State())

	_, err = breaker.Execute(func() (*pb.FilesReply, error) {
		return client.GetFiles(context.Background(), &pb.UserInfo{})
	})

	assert.ErrorIs(t, err, gobreaker.ErrOpenState)
}

func TestBreaker_SuccessAfterCooldown(t *testing.T) {
	srv := &testFilesServer{}
	srv.mode.Store(1) // always error

	addr, cleanup := startTestServer(t, srv)
	defer cleanup()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewUploaderClient(conn)
	breaker := newBreaker()

	for i := 0; i < consequtiveFailures; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, _ = breaker.Execute(func() (*pb.FilesReply, error) {
			return client.GetFiles(ctx, &pb.UserInfo{})
		})
		cancel()
	}

	assert.Equal(t, gobreaker.StateOpen, breaker.State())

	_, err = breaker.Execute(func() (*pb.FilesReply, error) {
		return client.GetFiles(context.Background(), &pb.UserInfo{})
	})

	assert.ErrorIs(t, err, gobreaker.ErrOpenState) // closed

	srv.mode.Store(0)

	time.Sleep(timeout)
	assert.Equal(t, gobreaker.StateHalfOpen, breaker.State()) // half-open after timeout

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond) // maxRequests trial calls allowed
	_, err = breaker.Execute(func() (*pb.FilesReply, error) {
		return client.GetFiles(ctx, &pb.UserInfo{})
	})
	cancel()

	assert.NoError(t, err)
	assert.Equal(t, gobreaker.StateClosed, breaker.State()) // finally closed
}
