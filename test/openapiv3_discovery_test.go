package test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOpenAPIv3DiscoveryRoot(t *testing.T) {
	client, extClient, _ := mustTestClient(t)
	b := &benchmark{
		Name: "openapi-v3-root-discovery",
	}
	err := setupCRDGroups(b, extClient)
	if err != nil {
		t.Fatalf("fail to setup: %v", err)
	}
	var firstRequestDuration time.Duration
	const T = 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), T)
	defer cancel()
	err = func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				start := time.Now()
				_, err := client.Discovery().OpenAPIV3().Paths()
				duration := time.Now().Sub(start)
				if firstRequestDuration == 0 {
					firstRequestDuration = duration
				}
				if err != nil {
					return nil
				}
			}
		}
	}()
	if err != nil {
		t.Errorf("fail to invoke root discovery: %v", err)
	}
	t.Logf("first request duration: %v", firstRequestDuration)
	err = tearDownCRDs(b, extClient)
	if err != nil {
		t.Errorf("fail to tear down: %v", err)
	}
}

func setupCRDGroups(b *benchmark, extClient clientset.Interface) error {
	const numGroups = 1000
	ctx := context.Background()
	objectCRD, err := loadYAMLTestData[apiextensionsv1.CustomResourceDefinition]("enforcement/objects.crd.yaml")
	if err != nil {
		return err
	}
	b.SetRandomLabel(&objectCRD.ObjectMeta)
	for i := 0; i < numGroups; i++ {
		crd := objectCRD.DeepCopy()
		group := fmt.Sprintf("test-%03d.%s", i, objectCRD.Spec.Group)
		crd.Name = fmt.Sprintf("%s.%s", strings.ToLower(objectCRD.Spec.Names.Plural), group)
		crd.Spec.Group = group
		_, err := extClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func tearDownCRDs(b *benchmark, extClient clientset.Interface) error {
	ctx := context.Background()
	err := extClient.ApiextensionsV1().CustomResourceDefinitions().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: b.LabelSelector()})
	return err
}
