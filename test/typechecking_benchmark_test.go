package test

import (
	"testing"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy"
	"k8s.io/apiserver/pkg/cel/openapi/resolver"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
)

func BenchmarkSchemaResolver(b *testing.B) {
	client, _, _ := mustTestClient(b)
	r := &resolver.ClientDiscoveryResolver{Discovery: client.Discovery()}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			deploymentGVK := appsv1.SchemeGroupVersion.WithKind("Deployment")
			s, err := r.ResolveSchema(deploymentGVK)
			if err != nil {
				b.Errorf("fail to resolve schema: %v", err)
			}
			if _, ok := s.Properties["spec"]; !ok {
				b.Errorf("returned schema is missing Spec")
			}
		}
	})
}

func BenchmarkTypeChecking(b *testing.B) {
	client, _, _ := mustTestClient(b)
	r := &resolver.ClientDiscoveryResolver{Discovery: client.Discovery()}
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(client.Discovery()))
	typeChecker := validatingadmissionpolicy.TypeChecker{
		SchemaResolver: r,
		RestMapper:     restMapper,
	}
	policy, err := loadYAMLTestData[admissionregistrationv1beta1.ValidatingAdmissionPolicy]("image-matches-namespace-environment.policy.yaml")
	if err != nil {
		b.Fatalf("fail to load test data: %v", err)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = typeChecker.Check(&policy)
		}
	})

}
