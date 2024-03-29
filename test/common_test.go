package test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/yaml"
)

const testLabel = "name.scalability.experiments.test.example.com"
const resourceSuffix = ".test.example.com"

type benchmark struct {
	uniqueLabel string
	Name        string
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

func testClient() (kubernetes.Interface, apiextensionsclientset.Interface, dynamic.Interface, error) {
	config, err := loadClientConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	config.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}
	extClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}
	return client, extClient, dynamicClient, nil
}

func mustTestClient(t testing.TB) (kubernetes.Interface, apiextensionsclientset.Interface, dynamic.Interface) {
	client, extClient, dynamicClient, err := testClient()
	if err != nil {
		t.Fatalf("fail to create client: %v", err)
	}
	return client, extClient, dynamicClient
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

func (b *benchmark) Label() string {
	if b.uniqueLabel == "" {
		b.uniqueLabel = fmt.Sprintf("%s-%s", b.Name, randId())
	}
	return b.uniqueLabel
}

func (b *benchmark) SetRandomLabel(meta *metav1.ObjectMeta) {
	if meta.Labels == nil {
		meta.Labels = make(map[string]string)
	}
	meta.Labels[testLabel] = b.Label()
}

func (b *benchmark) Run(ctx context.Context) (*benchmarkResult, error) {
	rootCtx, cancelCause := context.WithCancelCause(ctx)
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

func (b *benchmark) RandomResourceName() string {
	return fmt.Sprintf("%s-%s%s", b.Name, randId(), resourceSuffix)
}

func (b *benchmark) LabelSelector() string {
	return fmt.Sprintf("%s=%s", testLabel, b.Label())
}

func randId() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
