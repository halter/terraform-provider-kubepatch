// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"kubepatch": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccProtoV6ProviderFactoriesWithEcho includes the echo provider alongside the scaffolding provider.
// It allows for testing assertions on data returned by an ephemeral resource during Open.
// The echoprovider is used to arrange tests by echoing ephemeral data into the Terraform state.
// This lets the data be referenced in test assertions with state checks.
// var testAccProtoV6ProviderFactoriesWithEcho = map[string]func() (tfprotov6.ProviderServer, error){
// 	"scaffolding": providerserver.NewProtocol6WithError(New("test")()),
// 	"echo":        echoprovider.NewProviderServer(),
// }

func testAccPreCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
}

func providerConfig(t *testing.T) string {
	host := os.Getenv("KUBEPATCH_HOST")
	if host == "" {
		t.Fatal("KUBEPATCH_HOST must be set for acceptance tests")
	}
	clusterCaCertificate := os.Getenv("KUBEPATCH_CLUSTER_CA_CERTIFICATE")
	if clusterCaCertificate == "" {
		t.Fatal("KUBEPATCH_CLUSTER_CA_CERTIFICATE must be set for acceptance tests")
	}
	clientCertificate := os.Getenv("KUBEPATCH_CLIENT_CERTIFICATE")
	if clientCertificate == "" {
		t.Fatal("KUBEPATCH_CLIENT_CERTIFICATE must be set for acceptance tests")
	}
	clientKey := os.Getenv("KUBEPATCH_CLIENT_KEY")
	if clientKey == "" {
		t.Fatal("KUBEPATCH_CLIENT_KEY must be set for acceptance tests")
	}

	return fmt.Sprintf(`
provider "kubepatch" {
  host = "%[1]s"
  cluster_ca_certificate = base64decode("%[2]s")
  client_certificate = base64decode("%[3]s")
  client_key = base64decode("%[4]s")
}
`, host, clusterCaCertificate, clientCertificate, clientKey)
}
