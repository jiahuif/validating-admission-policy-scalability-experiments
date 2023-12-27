package test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

type benchmark struct {
	Concurrency int
	Duration    time.Duration
	Func        func(ctx context.Context, b *benchmark) error
}
type benchmarkResult struct {
	qps float64
}

func loadClientConfig() (*rest.Config, error) {
	// Connect to k8s
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here

	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	if config, err := kubeConfig.ClientConfig(); err == nil {
		return config, nil
	}

	// untested. assuming this is how it might work when run from inside clsuter
	return rest.InClusterConfig()
}

func testClient() (kubernetes.Interface, error) {
	config, err := loadClientConfig()
	if err != nil {
		return nil, err
	}
	config.Burst = 1024 * 1024
	config.QPS = float32(config.Burst)
	return kubernetes.NewForConfig(config)
}

func mustTestClient(t *testing.T) kubernetes.Interface {
	client, err := testClient()
	if err != nil {
		t.Fatalf("fail to create client: %v", err)
	}
	return client
}

func loadYAMLTestData[T any](name string) (T, error) {
	var object T
	fullName := "testdata/" + name
	_, err := os.Stat(fullName)
	if err != nil {
		fullName = "../testdata/" + name
		_, err := os.Stat(fullName)
		if err != nil {
			return object, err
		}
	}
	b, err := os.ReadFile(fullName)
	if err != nil {
		return object, err
	}
	err = yaml.Unmarshal(b, &object)
	return object, err
}

func randomResourceName(prefix, suffix string) string {
	return fmt.Sprintf("%s%d%s", prefix, rand.Int(), suffix)
}
func runConcurrentRequestsBenchmark(b *benchmark) (*benchmarkResult, error) {
	rootCtx, cancelCause := context.WithCancelCause(context.Background())
	defer cancelCause(nil)
	ctx, cancel := context.WithTimeout(context.Background(), b.Duration)
	defer cancel()
	startTime := time.Now()
	var numRequests atomic.Int64
	var wg sync.WaitGroup
	wg.Add(b.Concurrency)
	for i := 0; i < b.Concurrency; i++ {
		go func(ctx context.Context) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				func() {
					ctx, cancel := context.WithTimeout(ctx, time.Minute)
					defer cancel()
					err := b.Func(ctx, b)
					if err != nil && !errors.Is(err, context.Canceled) {
						cancelCause(fmt.Errorf("worker received following error: %v", err))
					}
				}()
				numRequests.Add(1)
			}
		}(ctx)
	}
	wg.Wait()
	select {
	case <-rootCtx.Done():
		return nil, context.Cause(rootCtx)
	default:
	}
	elapsed := time.Now().Sub(startTime)
	qps := (float64)(numRequests.Load()) / elapsed.Seconds()
	return &benchmarkResult{qps: qps}, nil
}
