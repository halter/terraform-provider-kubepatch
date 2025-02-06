// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// A generated module for TerraformProviderKubepatch functions
//
// This module has been generated via dagger init and serves as a reference to
// basic module structure as you get started with Dagger.
//
// Two functions have been pre-created. You can modify, delete, or add to them,
// as needed. They demonstrate usage of arguments and return types using simple
// echo and grep commands. The functions can be called from the dagger CLI or
// from one of the SDKs.
//
// The first line in this comment block is a short description line and the
// rest is a long description with more detail on the module's purpose or usage,
// if appropriate. All modules should have a short description.

package main

import (
	"context"
	"dagger/terraform-provider-kubepatch/internal/dagger"
	"strings"

	"gopkg.in/yaml.v3"
)

type TerraformProviderKubepatch struct {
	Sock       *dagger.Socket
	kubeconfig *KubeConfig
}

func New(sock *dagger.Socket) *TerraformProviderKubepatch {
	return &TerraformProviderKubepatch{
		Sock: sock,
	}
}

func (m *TerraformProviderKubepatch) BuildTestEnv(ctx context.Context, source *dagger.Directory) *dagger.Container {
	goCache := dag.CacheVolume("go")
	goModCache := dag.CacheVolume("go-mod")
	container := dag.Container().
		From("golang:1.23").
		WithUnixSocket("/var/run/docker.sock", m.Sock).
		WithExec([]string{"apt", "update"}).
		WithExec([]string{"apt", "install", "-y", "docker.io", "jq"}).
		WithMountedCache("/root/.cache/go-build", goCache).
		WithMountedCache("/root/go/pkg/mod", goModCache).
		WithExec([]string{"go", "install", "sigs.k8s.io/kind@v0.26.0"}).
		WithExec([]string{"kind", "create", "cluster"}, dagger.ContainerWithExecOpts{Expect: dagger.ReturnTypeAny}).
		WithDirectory("/src", source).
		WithWorkdir("/src")

	daggerEngineContainerName, _ := container.WithExec([]string{"bash", "-c", `docker ps --format '{{json .}}' | jq -r '. | select(.Image|startswith("registry.dagger.io/engine:")) | .Names'`}).Stdout(ctx)
	daggerEngineContainerName = strings.TrimSpace(daggerEngineContainerName)
	container.WithExec([]string{"bash", "-c", "docker network connect kind " + daggerEngineContainerName + " || true"}).Stdout(ctx)

	data, _ := container.WithExec([]string{"kind", "get", "kubeconfig"}).Stdout(ctx)

	kindControlPlainContainerID, _ := container.WithExec([]string{"bash", "-c", `docker ps --format '{{json .}}' | jq -r '. | select(.Names=="kind-control-plane") | .ID'`}).Stdout(ctx)
	kindControlPlainContainerID = strings.TrimSpace(kindControlPlainContainerID)
	kindControlPlainContainerIP, _ := container.WithExec([]string{"bash", "-c", `docker inspect ` + kindControlPlainContainerID + ` | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress'`}).Stdout(ctx)
	kindControlPlainContainerIP = strings.TrimSpace(kindControlPlainContainerIP)

	kubeConfig := KubeConfig{}
	_ = yaml.Unmarshal([]byte(data), &kubeConfig)

	kubeConfig.Clusters[0].Cluster.Server = "https://" + kindControlPlainContainerIP + ":6443"

	m.kubeconfig = &kubeConfig

	return container.
		WithEnvVariable("KUBEPATCH_HOST", "https://"+kindControlPlainContainerIP+":6443").
		WithEnvVariable("KUBEPATCH_CLUSTER_CA_CERTIFICATE", kubeConfig.Clusters[0].Cluster.CertificateAuthorityData).
		WithEnvVariable("KUBEPATCH_CLIENT_CERTIFICATE", kubeConfig.Users[0].User.ClientCertificateData).
		WithEnvVariable("KUBEPATCH_CLIENT_KEY", kubeConfig.Users[0].User.ClientKeyData)
}

func (m *TerraformProviderKubepatch) SetupTestFixtures(ctx context.Context, source *dagger.Directory) *dagger.Container {
	container := m.BuildTestEnv(ctx, source)

	b, _ := yaml.Marshal(m.kubeconfig)
	kubeConfig := string(b)

	dag.Container().
		From("bitnami/kubectl:1.32.1").
		WithDirectory("/src", source).
		WithNewFile("/.kube/config", kubeConfig).
		WithExec([]string{"kubectl", "config", "use-context", "kind-kind"}).
		WithExec([]string{"kubectl", "apply", "--validate=false", "-f", "/src/dagger/fixtures/opentelemetry-operator-controller-manager.yaml"}).Stdout(ctx)

	return container
}

func (m *TerraformProviderKubepatch) Test(ctx context.Context, source *dagger.Directory) {
	c := m.SetupTestFixtures(ctx, source)
	c.WithExec([]string{"make", "testacc"}).Stdout(ctx)
	return
}
