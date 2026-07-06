package aprs

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
)

func TestMain(m *testing.M) {
	_ = log.Init(false)
	os.Exit(m.Run())
}

// fakeProvider is a minimal ConfigProvider that returns a settable device list.
// It implements the methods the worker touches (GetDevices for reconciliation,
// GetDevice for per-cycle re-reads and report-interval lookups); the rest are
// promoted from the (nil) embedded interface and must not be called.
type fakeProvider struct {
	config.ConfigProvider
	mu      sync.Mutex
	devices []config.DeviceData
}

func (f *fakeProvider) set(devices []config.DeviceData) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.devices = devices
}

func (f *fakeProvider) GetDevices() ([]config.DeviceData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]config.DeviceData, len(f.devices))
	copy(out, f.devices)
	return out, nil
}

func (f *fakeProvider) GetDevice(name string) (*config.DeviceData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.devices {
		if f.devices[i].Name == name {
			d := f.devices[i]
			return &d, nil
		}
	}
	return nil, fmt.Errorf("device %s not found", name)
}

func readyDevice(name string) config.DeviceData {
	return config.DeviceData{
		Name:         name,
		APRSEnabled:  true,
		APRSCallsign: "N0CALL",
		Latitude:     45.0,
		Longitude:    -122.0,
	}
}

func TestReconcileWorkersEnableDisable(t *testing.T) {
	fp := &fakeProvider{}
	a := &Controller{configProvider: fp}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	workers := make(map[string]context.CancelFunc)

	// No devices enabled: no workers.
	a.reconcileWorkers(ctx, &wg, workers)
	if len(workers) != 0 {
		t.Fatalf("expected 0 workers, got %d", len(workers))
	}

	// One APRS-ready device: a worker starts.
	fp.set([]config.DeviceData{readyDevice("station1")})
	a.reconcileWorkers(ctx, &wg, workers)
	if _, ok := workers["station1"]; !ok || len(workers) != 1 {
		t.Fatalf("expected worker for station1, got %v", keys(workers))
	}

	// Reconcile again with the same device: no duplicate worker.
	a.reconcileWorkers(ctx, &wg, workers)
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker after idempotent reconcile, got %d", len(workers))
	}

	// Device disabled: its worker is stopped and removed.
	disabled := readyDevice("station1")
	disabled.APRSEnabled = false
	fp.set([]config.DeviceData{disabled})
	a.reconcileWorkers(ctx, &wg, workers)
	if len(workers) != 0 {
		t.Fatalf("expected worker removed after disable, got %v", keys(workers))
	}

	// Stop any goroutines that started, then wait for them to exit.
	cancel()
	wg.Wait()
}

func keys(m map[string]context.CancelFunc) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
