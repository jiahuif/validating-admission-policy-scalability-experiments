package test

import (
	"context"
	"errors"
	"testing"
	"time"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConcurrentPolicyCreation(t *testing.T) {
	var C = 8
	var D = time.Minute
	ctx := context.Background()
	policy, err := loadYAMLTestData[admissionregistrationv1beta1.ValidatingAdmissionPolicy]("image-matches-namespace-environment.policy.yaml")
	if err != nil {
		t.Fatalf("fail to load test data: %v", err)
	}
	client := mustTestClient(t)
	b := &benchmark{
		Concurrency: C,
		Duration:    D,
		Name:        "creation",
		Func: func(ctx context.Context, b *benchmark) error {
			policy := policy.DeepCopy()
			policy.Name = b.RandomResourceName()
			_, err := client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{})
			// ignore timeout for benchmark
			if errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		},
	}
	r, err := b.Run(context.Background())
	if err != nil {
		t.Fatalf("fail to create: %v", err)
	}
	t.Logf("QPS: %f", r.qps)
	err = client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: b.LabelSelector(),
	})
	if err != nil {
		t.Fatalf("fail to cleanup: %v", err)
	}
}