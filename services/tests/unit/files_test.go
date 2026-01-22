package services

import (
	"errors"
	"log"
	"os"
	"testing"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/sony/gobreaker/v2"
)

var (
	cb                  *gobreaker.CircuitBreaker[*pb.FilesReply]
	maxRequests         = 5
	consequtiveFailures = 3
)

func TestMain(m *testing.M) {
	cb = gobreaker.NewCircuitBreaker[*pb.FilesReply](gobreaker.Settings{
		Name:        "test",
		MaxRequests: uint32(maxRequests),
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= uint32(consequtiveFailures)
		},

		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("circuit breaker %s: %s → %s", name, from, to)
		},
	})

	os.Exit(m.Run())
}

func TestFilesCircuitBreaker_Succcess(t *testing.T) {
	success := func() (*pb.FilesReply, error) {
		return &pb.FilesReply{}, nil
	}

	for i := 0; i < consequtiveFailures*2; i++ {
		_, err := cb.Execute(success)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if cb.State() != gobreaker.StateClosed {
		t.Fatalf("expected breaker to remain closed, got %v", cb.State())
	}
}

func TestFilesCircuitBreaker_TripsAfterFailures(t *testing.T) {
	fail := func() (*pb.FilesReply, error) {
		return nil, errors.New("grpc failure")
	}

	for i := 0; i < consequtiveFailures; i++ {
		_, _ = cb.Execute(fail)
	}

	if cb.State() != gobreaker.StateOpen {
		t.Fatalf("expected breaker to be open, got %v", cb.State())
	}
}

func TestFilesCircuitBreaker_FastFail(t *testing.T) {
	localCb := gobreaker.NewCircuitBreaker[*pb.FilesReply](gobreaker.Settings{
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= 1
		},
	})

	fail := func() (*pb.FilesReply, error) {
		return nil, errors.New("grpc failure")
	}

	_, _ = localCb.Execute(fail)

	_, err := localCb.Execute(fail)
	if err == nil {
		t.Fatal("expected fast-fail error, got nil")
	}

	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected ErrOpenState, got %v", err)
	}
}
