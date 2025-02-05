// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/mitchellh/go-homedir"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	apimachineryschema "k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Ensure KubernetesPatchProvider satisfies various provider interfaces.
var _ provider.Provider = &KubernetesPatchProvider{}
var _ provider.ProviderWithFunctions = &KubernetesPatchProvider{}
var _ provider.ProviderWithEphemeralResources = &KubernetesPatchProvider{}

// KubernetesPatchProvider defines the provider implementation.
type KubernetesPatchProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// KubernetesPatchProviderModel describes the provider data model.
type KubernetesPatchProviderModel struct {
	Host     types.String `tfsdk:"host"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Insecure types.Bool   `tfsdk:"insecure"`

	TLSServerName        types.String `tfsdk:"tls_server_name"`
	ClientCertificate    types.String `tfsdk:"client_certificate"`
	ClientKey            types.String `tfsdk:"client_key"`
	ClusterCACertificate types.String `tfsdk:"cluster_ca_certificate"`

	ConfigPaths []types.String `tfsdk:"config_paths"`
	ConfigPath  types.String   `tfsdk:"config_path"`

	ConfigContext         types.String `tfsdk:"config_context"`
	ConfigContextAuthInfo types.String `tfsdk:"config_context_auth_info"`
	ConfigContextCluster  types.String `tfsdk:"config_context_cluster"`

	Token types.String `tfsdk:"token"`

	ProxyURL types.String `tfsdk:"proxy_url"`

	IgnoreAnnotations types.List `tfsdk:"ignore_annotations"`
	IgnoreLabels      types.List `tfsdk:"ignore_labels"`

	Exec []struct {
		APIVersion types.String            `tfsdk:"api_version"`
		Command    types.String            `tfsdk:"command"`
		Env        map[string]types.String `tfsdk:"env"`
		Args       []types.String          `tfsdk:"args"`
	} `tfsdk:"exec"`

	Experiments []struct {
		ManifestResource types.Bool `tfsdk:"manifest_resource"`
	} `tfsdk:"experiments"`
}

func (p *KubernetesPatchProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kubernetes_patch"
	resp.Version = p.version
}

func (p *KubernetesPatchProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "The hostname (in form of URI) of Kubernetes master.",
				Optional:    true,
			},
			"username": schema.StringAttribute{
				Description: "The username to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "The password to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
				Optional:    true,
			},
			"insecure": schema.BoolAttribute{
				Description: "Whether server should be accessed without verifying the TLS certificate.",
				Optional:    true,
			},
			"tls_server_name": schema.StringAttribute{
				Description: "Server name passed to the server for SNI and is used in the client to check server certificates against.",
				Optional:    true,
			},
			"client_certificate": schema.StringAttribute{
				Description: "PEM-encoded client certificate for TLS authentication.",
				Optional:    true,
			},
			"client_key": schema.StringAttribute{
				Description: "PEM-encoded client certificate key for TLS authentication.",
				Optional:    true,
			},
			"cluster_ca_certificate": schema.StringAttribute{
				Description: "PEM-encoded root certificates bundle for TLS authentication.",
				Optional:    true,
			},
			"config_paths": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "A list of paths to kube config files. Can be set with KUBE_CONFIG_PATHS environment variable.",
				Optional:    true,
			},
			"config_path": schema.StringAttribute{
				Description: "Path to the kube config file. Can be set with KUBE_CONFIG_PATH.",
				Optional:    true,
			},
			"config_context": schema.StringAttribute{
				Description: "",
				Optional:    true,
			},
			"config_context_auth_info": schema.StringAttribute{
				Description: "",
				Optional:    true,
			},
			"config_context_cluster": schema.StringAttribute{
				Description: "",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "Token to authenticate an service account",
				Optional:    true,
			},
			"proxy_url": schema.StringAttribute{
				Description: "URL to the proxy to be used for all API requests",
				Optional:    true,
			},
			"ignore_annotations": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "List of Kubernetes metadata annotations to ignore across all resources handled by this provider for situations where external systems are managing certain resource annotations. Each item is a regular expression.",
				Optional:    true,
			},
			"ignore_labels": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "List of Kubernetes metadata labels to ignore across all resources handled by this provider for situations where external systems are managing certain resource labels. Each item is a regular expression.",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"exec": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"api_version": schema.StringAttribute{
							Required: true,
						},
						"command": schema.StringAttribute{
							Required: true,
						},
						"env": schema.MapAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"args": schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
					},
				},
			},
			"experiments": schema.ListNestedBlock{
				Description: "Enable and disable experimental features.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"manifest_resource": schema.BoolAttribute{
							Description:        "Enable the `kubernetes_manifest` resource.",
							Optional:           true,
							DeprecationMessage: "The kubernetes_manifest resource is now permanently enabled and no longer considered an experiment. This flag has no effect and will be removed in the near future.",
						},
					},
				},
			},
		},
	}
}

func (p *KubernetesPatchProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data KubernetesPatchProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	// if data.Endpoint.IsNull() { /* ... */ }

	cfg := &rest.Config{}
	if !data.Host.IsNull() {
		cfg.Host = data.Host.String()
	}
	if !data.ClusterCACertificate.IsNull() {
		cfg.CAData
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		resp.Diagnostics.AddError("could not get clientset", err.Error())
		return
	}

	// Example client configuration for data sources and resources
	client := http.DefaultClient
	resp.DataSourceData = client
	resp.ResourceData = clientset
}

func (p *KubernetesPatchProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
	}
}

func (p *KubernetesPatchProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewExampleEphemeralResource,
	}
}

func (p *KubernetesPatchProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewExampleDataSource,
	}
}

func (p *KubernetesPatchProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewExampleFunction,
	}
}

func New(version string, sdkv2Meta func() any) func() provider.Provider {
	return func() provider.Provider {
		return &KubernetesPatchProvider{
			version: version,
		}
	}
}

func initializeConfiguration(d KubernetesPatchProviderModel) (*restclient.Config, diag.Diagnostics) {
	diags := make(diag.Diagnostics, 0)
	overrides := &clientcmd.ConfigOverrides{}
	loader := &clientcmd.ClientConfigLoadingRules{}

	configPaths := []string{}

	if v := d.ConfigPath.ValueStringPointer(); v != nil {
		configPaths = []string{*v}
	} else if len(d.ConfigPaths) > 0 {
		for _, p := range d.ConfigPaths {
			configPaths = append(configPaths, p.String())
		}
	} else if v := os.Getenv("KUBE_CONFIG_PATHS"); v != "" {
		// NOTE we have to do this here because the schema
		// does not yet allow you to set a default for a TypeList
		configPaths = filepath.SplitList(v)
	}

	if len(configPaths) > 0 {
		expandedPaths := []string{}
		for _, p := range configPaths {
			path, err := homedir.Expand(p)
			if err != nil {
				return nil, append(diags, diag.FromErr(err)...)
			}

			log.Printf("[DEBUG] Using kubeconfig: %s", path)
			expandedPaths = append(expandedPaths, path)
		}

		if len(expandedPaths) == 1 {
			loader.ExplicitPath = expandedPaths[0]
		} else {
			loader.Precedence = expandedPaths
		}

		ctxSuffix := "; default context"

		kubectx := d.ConfigContext.ValueStringPointer()
		authInfo := d.ConfigContextAuthInfo.ValueStringPointer()
		cluster := d.ConfigContextCluster.ValueStringPointer()
		if kubectx != nil || authInfo != nil || cluster != nil {
			ctxSuffix = "; overridden context"
			if kubectx != nil {
				overrides.CurrentContext = *kubectx
				ctxSuffix += fmt.Sprintf("; config ctx: %s", overrides.CurrentContext)
				log.Printf("[DEBUG] Using custom current context: %q", overrides.CurrentContext)
			}

			overrides.Context = clientcmdapi.Context{}
			if authInfo != nil {
				overrides.Context.AuthInfo = *authInfo
				ctxSuffix += fmt.Sprintf("; auth_info: %s", overrides.Context.AuthInfo)
			}
			if cluster != nil {
				overrides.Context.Cluster = *cluster
				ctxSuffix += fmt.Sprintf("; cluster: %s", overrides.Context.Cluster)
			}
			log.Printf("[DEBUG] Using overridden context: %#v", overrides.Context)
		}
	}

	// Overriding with static configuration
	if v := d.Insecure.ValueBoolPointer(); v != nil {
		overrides.ClusterInfo.InsecureSkipTLSVerify = *v
	}
	if v := d.TLSServerName.ValueStringPointer(); v != nil {
		overrides.ClusterInfo.TLSServerName = *v
	}
	if v := d.ClusterCACertificate.ValueStringPointer(); v != nil {
		overrides.ClusterInfo.CertificateAuthorityData = bytes.NewBufferString(*v).Bytes()
	}
	if v := d.ClientCertificate.ValueStringPointer(); v != nil {
		overrides.AuthInfo.ClientCertificateData = bytes.NewBufferString(*v).Bytes()
	}
	if v := d.Host.ValueStringPointer(); v != nil {
		// Server has to be the complete address of the kubernetes cluster (scheme://hostname:port), not just the hostname,
		// because `overrides` are processed too late to be taken into account by `defaultServerUrlFor()`.
		// This basically replicates what defaultServerUrlFor() does with config but for overrides,
		// see https://github.com/kubernetes/client-go/blob/v12.0.0/rest/url_utils.go#L85-L87
		hasCA := len(overrides.ClusterInfo.CertificateAuthorityData) != 0
		hasCert := len(overrides.AuthInfo.ClientCertificateData) != 0
		defaultTLS := (hasCA || hasCert) && !overrides.ClusterInfo.InsecureSkipTLSVerify
		host, _, err := restclient.DefaultServerURL(*v, "", apimachineryschema.GroupVersion{}, defaultTLS)
		if err != nil {
			nd := diag.Diagnostic{
				Severity:      diag.Error,
				Summary:       fmt.Sprintf("Failed to parse value for host: %s", *v),
				Detail:        err.Error(),
				AttributePath: cty.Path{}.IndexString("host"),
			}
			return nil, append(diags, nd)
		}
		overrides.ClusterInfo.Server = host.String()
	}
	if v := d.Username.ValueStringPointer(); v != nil {
		overrides.AuthInfo.Username = *v
	}
	if v := d.Password.ValueStringPointer(); v != nil {
		overrides.AuthInfo.Password = *v
	}
	if v := d.ClientKey.ValueStringPointer(); v != nil {
		overrides.AuthInfo.ClientKeyData = bytes.NewBufferString(*v).Bytes()
	}
	if v := d.Token.ValueStringPointer(); v != nil {
		overrides.AuthInfo.Token = *v
	}

	if len(d.Exec) > 0 {
		v := d.Exec
		exec := &clientcmdapi.ExecConfig{}
		spec := v[0]
		exec.InteractiveMode = clientcmdapi.IfAvailableExecInteractiveMode
		exec.APIVersion = spec.APIVersion.String()
		exec.Command = spec.Command.String()
		exec.Args = expandStringSliceV2(spec.Args)
		for kk, vv := range spec.Env {
			exec.Env = append(exec.Env, clientcmdapi.ExecEnvVar{Name: kk, Value: vv.String()})
		}
		overrides.AuthInfo.Exec = exec
	}

	if v := d.ProxyURL.ValueStringPointer(); v != nil {
		overrides.ClusterDefaults.ProxyURL = *v
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		nd := diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  "Provider was supplied an invalid configuration. Further operations likely to fail.",
			Detail:   err.Error(),
		}
		log.Printf("[WARN] Provider was supplied an invalid configuration. Further operations likely to fail: %v", err)
		return nil, append(diags, nd)
	}

	return cfg, diags
}

func expandStringSlice(s []interface{}) []string {
	result := make([]string, len(s))
	for k, v := range s {
		// Handle the Terraform parser bug which turns empty strings in lists to nil.
		if v == nil {
			result[k] = ""
		} else {
			result[k] = v.(string)
		}
	}
	return result
}

func expandStringSliceV2(s []types.String) []string {
	result := make([]string, len(s))
	for k, v := range s {
		result[k] = v.String()
	}
	return result
}
