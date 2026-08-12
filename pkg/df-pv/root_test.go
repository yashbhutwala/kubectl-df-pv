package df_pv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestConvertQuantityValueToHumanReadableIECString(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero", bytes: 0, want: "0"},
		{name: "lt_1Ki", bytes: 512, want: "512"},
		{name: "eq_1Ki", bytes: 1 << 10, want: "1Ki"},
		{name: "one_point_five_Ki", bytes: 1536, want: "1.5Ki"},
		{name: "eq_1Mi", bytes: 1 << 20, want: "1Mi"},
		{name: "eq_1Gi", bytes: 1 << 30, want: "1Gi"},
		{name: "one_point_five_Gi", bytes: (1 << 30) + (512 << 20), want: "1.5Gi"},
		{name: "eq_1Ti", bytes: 1 << 40, want: "1Ti"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := resource.NewQuantity(tt.bytes, resource.BinarySI)
			got := ConvertQuantityValueToHumanReadableIECString(q)
			if got != tt.want {
				t.Fatalf("ConvertQuantityValueToHumanReadableIECString(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestProduceOutputRowsConcurrentlySkipsEmptyNodeNames(t *testing.T) {
	clientset, requestCount := newTestClientset(t, http.StatusInternalServerError)
	outputRowPVCChan := make(chan *OutputRowPVC)
	result := make(chan error, 1)

	go func() {
		result <- ProduceOutputRowsConcurrently(context.Background(), clientset, "", []string{""}, outputRowPVCChan)
	}()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("ProduceOutputRowsConcurrently returned an unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ProduceOutputRowsConcurrently did not return for an empty node name")
	}

	select {
	case _, ok := <-outputRowPVCChan:
		if ok {
			t.Fatal("output row channel should be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("output row channel was not closed")
	}

	if *requestCount != 0 {
		t.Fatalf("expected no node stats requests, got %d", *requestCount)
	}
}

func TestProduceOutputRowsConcurrentlyClosesChannelOnNodeError(t *testing.T) {
	clientset, requestCount := newTestClientset(t, http.StatusInternalServerError)
	outputRowPVCChan := make(chan *OutputRowPVC)
	result := make(chan error, 1)

	go func() {
		result <- ProduceOutputRowsConcurrently(context.Background(), clientset, "", []string{"node-a"}, outputRowPVCChan)
	}()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("ProduceOutputRowsConcurrently returned nil for a node request error")
		}
	case <-time.After(time.Second):
		t.Fatal("ProduceOutputRowsConcurrently did not return after a node request error")
	}

	select {
	case _, ok := <-outputRowPVCChan:
		if ok {
			t.Fatal("output row channel should be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("output row channel was not closed after a node request error")
	}

	if *requestCount != 1 {
		t.Fatalf("expected one node stats request, got %d", *requestCount)
	}
}

func newTestClientset(t *testing.T, statusCode int) (*kubernetes.Clientset, *int) {
	t.Helper()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		http.Error(w, "test node error", statusCode)
	}))
	t.Cleanup(server.Close)

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("failed to create test clientset: %v", err)
	}

	return clientset, &requestCount
}
