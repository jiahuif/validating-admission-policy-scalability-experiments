package test

import (
	"context"
	"testing"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var objectGVR = schema.GroupVersionResource{
	Group:    "policycreation.example.com",
	Version:  "v1",
	Resource: "objects",
}

var paramGVR = schema.GroupVersionResource{
	Group:    "policycreation.example.com",
	Version:  "v1",
	Resource: "params",
}

const namespace = "default"

func TestPolicyEnforcement(t *testing.T) {
	client, extclient, dynamicClient := mustTestClient(t)
	b := &benchmark{Name: "policy-enforcement"}
	err := policyEnforcementSetup(b, client, extclient, dynamicClient)
	if err != nil {
		t.Errorf("fail to setup: %v", err)
	}
	err = policyEnforcementTeardown(b, client, extclient, dynamicClient)
	if err != nil {
		t.Errorf("fail to tear down: %v", err)
	}
}

func policyEnforcementSetup(b *benchmark, client kubernetes.Interface, extClient apiextensionsclientset.Interface, dynamicClient dynamic.Interface) error {
	ctx := context.Background()
	objectCRD, err := loadYAMLTestData[apiextensionsv1.CustomResourceDefinition]("enforcement/objects.crd.yaml")
	if err != nil {
		return err
	}
	b.SetRandomLabel(&objectCRD.ObjectMeta)
	_, err = extClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, &objectCRD, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	paramsCRD, err := loadYAMLTestData[apiextensionsv1.CustomResourceDefinition]("enforcement/params.crd.yaml")
	if err != nil {
		return err
	}
	b.SetRandomLabel(&paramsCRD.ObjectMeta)
	_, err = extClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, &paramsCRD, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	policy, err := loadYAMLTestData[admissionregistrationv1beta1.ValidatingAdmissionPolicy]("enforcement/demo-policy-with-params.policy.yaml")
	if err != nil {
		return err
	}
	b.SetRandomLabel(&policy.ObjectMeta)
	_, err = client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Create(ctx, &policy, metav1.CreateOptions{})
	binding, err := loadYAMLTestData[admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding]("enforcement/demo-policy-with-params.binding.yaml")
	if err != nil {
		return err
	}
	b.SetRandomLabel(&binding.ObjectMeta)
	_, err = client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Create(ctx, &binding, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	paramObject, err := loadYAMLTestData[unstructured.Unstructured]("enforcement/param.params.yaml")
	if err != nil {
		return err
	}
	paramObject.Object["metadata"].(map[string]any)["labels"] = map[string]any{
		testLabel: b.Label(),
	}
	_, err = dynamicClient.Resource(paramGVR).Namespace(namespace).Create(ctx, &paramObject, metav1.CreateOptions{})
	return err
}

func policyEnforcementTeardown(b *benchmark, client kubernetes.Interface, extClient apiextensionsclientset.Interface, dynamicClient dynamic.Interface) error {
	ctx := context.Background()
	_ = dynamicClient.Resource(paramGVR).Namespace(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: b.LabelSelector()})
	_ = dynamicClient.Resource(objectGVR).Namespace(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: b.LabelSelector()})
	_ = client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: b.LabelSelector()})
	_ = client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: b.LabelSelector()})
	_ = extClient.ApiextensionsV1().CustomResourceDefinitions().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: b.LabelSelector()})
	return nil
}
