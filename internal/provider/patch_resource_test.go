// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

func getClientSet() (*kubernetes.Clientset, error) {
	caData, err := base64.StdEncoding.DecodeString(os.Getenv("KUBEPATCH_CLUSTER_CA_CERTIFICATE"))
	if err != nil {
		return nil, err
	}
	clientCertificate, err := base64.StdEncoding.DecodeString(os.Getenv("KUBEPATCH_CLIENT_CERTIFICATE"))
	if err != nil {
		return nil, err
	}
	keyData, err := base64.StdEncoding.DecodeString(os.Getenv("KUBEPATCH_CLIENT_KEY"))
	if err != nil {
		return nil, err
	}
	restClient := &restclient.Config{
		Host: os.Getenv("KUBEPATCH_HOST"),
		TLSClientConfig: restclient.TLSClientConfig{
			CAData:   caData,
			CertData: clientCertificate,
			KeyData:  keyData,
		},
	}
	return kubernetes.NewForConfig(restClient)
}

func TestAccPatchResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPatchResourceConfig(t),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"kubepatch_patch.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("example-id"),
					),
				},
				Check: func(state *terraform.State) error {
					clientset, err := getClientSet()
					if err != nil {
						return err
					}

					deployment, err := clientset.AppsV1().Deployments("default").Get(context.TODO(), "opentelemetry-operator-controller-manager", metav1.GetOptions{})
					if err != nil {
						return err
					}

					expectedArgs := []string{"--metrics-addr=127.0.0.1:8080", "--enable-leader-election", "--zap-log-level=info", "--zap-time-encoding=rfc3339nano", "--enable-nginx-instrumentation=true", "--enable-go-instrumentation=true"}
					if len(deployment.Spec.Template.Spec.Containers[0].Args) != len(expectedArgs) {
						return fmt.Errorf("expected %d args, got %d", len(expectedArgs), len(deployment.Spec.Template.Spec.Containers[0].Args))
					}
					for i, arg := range deployment.Spec.Template.Spec.Containers[0].Args {
						if arg != expectedArgs[i] {
							return fmt.Errorf("expected arg %d to be %q, got %q", i, expectedArgs[i], arg)
						}
					}
					return nil
				},
			},
			// Update and Read testing
			{
				Config: testAccPatchResourceConfigUpdate(t),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"kubepatch_patch.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("example-id"),
					),
				},
				Check: func(state *terraform.State) error {
					clientset, err := getClientSet()
					if err != nil {
						return err
					}

					deployment, err := clientset.AppsV1().Deployments("default").Get(context.TODO(), "opentelemetry-operator-controller-manager", metav1.GetOptions{})
					if err != nil {
						return err
					}

					expectedArgs := []string{"--metrics-addr=127.0.0.1:8080", "--enable-leader-election", "--zap-log-level=info", "--zap-time-encoding=rfc3339nano", "--enable-nginx-instrumentation=true", "--enable-go-instrumentation=true", "enable-dotnet-instrumentation=true"}
					if len(deployment.Spec.Template.Spec.Containers[0].Args) != len(expectedArgs) {
						return fmt.Errorf("expected %d args, got %d", len(expectedArgs), len(deployment.Spec.Template.Spec.Containers[0].Args))
					}
					for i, arg := range deployment.Spec.Template.Spec.Containers[0].Args {
						if arg != expectedArgs[i] {
							return fmt.Errorf("expected arg %d to be %q, got %q", i, expectedArgs[i], arg)
						}
					}
					return nil
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccPatchResourceConfig(t *testing.T) string {
	return providerConfig(t) + `
resource "kubepatch_patch" "test" {
  namespace = "default"
  resource = "deployments"
  name = "opentelemetry-operator-controller-manager"
  type = "json"
  data = jsonencode([
    {
      op = "replace"
      path = "/spec/template/spec/containers/0/args"
      value = ["--metrics-addr=127.0.0.1:8080", "--enable-leader-election", "--zap-log-level=info", "--zap-time-encoding=rfc3339nano", "--enable-nginx-instrumentation=true", "--enable-go-instrumentation=true"]
    },
  ])
}
`
}

func testAccPatchResourceConfigUpdate(t *testing.T) string {
	return providerConfig(t) + `
resource "kubepatch_patch" "test" {
  namespace = "default"
  resource = "deployments"
  name = "opentelemetry-operator-controller-manager"
  type = "json"
  data = jsonencode([
    {
      op = "replace"
      path = "/spec/template/spec/containers/0/args"
      value = ["--metrics-addr=127.0.0.1:8080", "--enable-leader-election", "--zap-log-level=info", "--zap-time-encoding=rfc3339nano", "--enable-nginx-instrumentation=true", "--enable-go-instrumentation=true", "enable-dotnet-instrumentation=true"]
    },
  ])
}
`
}
