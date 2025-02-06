// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PatchResource{}
var _ resource.ResourceWithImportState = &PatchResource{}

func NewPatchResource() resource.Resource {
	return &PatchResource{}
}

// PatchResource defines the resource implementation.
type PatchResource struct {
	client *kubernetes.Clientset
}

// PatchResourceModel describes the resource data model.
type PatchResourceModel struct {
	Namespace types.String `tfsdk:"namespace"`
	Resource  types.String `tfsdk:"resource"`
	Name      types.String `tfsdk:"name"`
	Type      types.String `tfsdk:"type"`
	Data      types.String `tfsdk:"data"`
	Id        types.String `tfsdk:"id"`
}

func (r *PatchResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_patch"
}

func (r *PatchResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Patch resource",

		Attributes: map[string]schema.Attribute{
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
				Required:            true,
			},
			"resource": schema.StringAttribute{
				MarkdownDescription: "Kubernetes API resource",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"bindings",
						"componentstatuses",
						"configmaps",
						"endpoints",
						"events",
						"limitranges",
						"namespaces",
						"nodes",
						"persistentvolumeclaims",
						"persistentvolumes",
						"pods",
						"podtemplates",
						"replicationcontrollers",
						"resourcequotas",
						"secrets",
						"serviceaccounts",
						"services",
						"controllerrevisions",
						"daemonsets",
						"deployments",
						"replicasets",
						"statefulsets",
						"cronjobs",
						"jobs",
					),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Kubernetes API resource name",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of patch being provided; one of [json merge strategic]",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("json", "merge", "strategic"),
				},
			},
			"data": schema.StringAttribute{
				MarkdownDescription: "The patch to be applied to the resource JSON file.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *PatchResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*kubernetes.Clientset)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *PatchResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PatchResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create example, got error: %s", err))
	//     return
	// }

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	data.Id = types.StringValue("example-id")

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	err := r.patch(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to patch, got error: %s", err))
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PatchResource) patch(ctx context.Context, data PatchResourceModel) error {
	var pt k8stypes.PatchType
	switch t := data.Type.ValueString(); t {
	case "json":
		pt = k8stypes.JSONPatchType
	case "merge":
		pt = k8stypes.MergePatchType
	case "strategic":
		pt = k8stypes.StrategicMergePatchType
	}

	namespace := data.Namespace.ValueString()

	var err error
	switch res := data.Resource.ValueString(); res {
	case "componentstatuses":
		_, err = r.client.CoreV1().ComponentStatuses().Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "configmaps":
		_, err = r.client.CoreV1().ConfigMaps(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "endpoints":
		_, err = r.client.CoreV1().Endpoints(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "events":
		_, err = r.client.CoreV1().Events(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "limitranges":
		_, err = r.client.CoreV1().LimitRanges(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "namespaces":
		_, err = r.client.CoreV1().Namespaces().Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "nodes":
		_, err = r.client.CoreV1().Nodes().Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "persistentvolumeclaims":
		_, err = r.client.CoreV1().PersistentVolumeClaims(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "persistentvolumes":
		_, err = r.client.CoreV1().PersistentVolumes().Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "pods":
		_, err = r.client.CoreV1().Pods(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "podtemplates":
		_, err = r.client.CoreV1().PodTemplates(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "replicationcontrollers":
		_, err = r.client.CoreV1().ReplicationControllers(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "resourcequotas":
		_, err = r.client.CoreV1().ResourceQuotas(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "secrets":
		_, err = r.client.CoreV1().Secrets(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "serviceaccounts":
		_, err = r.client.CoreV1().ServiceAccounts(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "services":
		_, err = r.client.CoreV1().Services(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "controllerrevisions":
		_, err = r.client.AppsV1().ControllerRevisions(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "daemonsets":
		_, err = r.client.AppsV1().DaemonSets(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "deployments":
		_, err = r.client.AppsV1().Deployments(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "replicasets":
		_, err = r.client.AppsV1().ReplicaSets(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "statefulsets":
		_, err = r.client.AppsV1().StatefulSets(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "cronjobs":
		_, err = r.client.BatchV1().CronJobs(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	case "jobs":
		_, err = r.client.BatchV1().Jobs(namespace).Patch(ctx, data.Name.ValueString(), pt, []byte(data.Data.ValueString()), metav1.PatchOptions{})
	}

	return err
}

func (r *PatchResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PatchResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read example, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PatchResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PatchResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update example, got error: %s", err))
	//     return
	// }
	err := r.patch(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to patch, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PatchResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PatchResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete example, got error: %s", err))
	//     return
	// }
}

func (r *PatchResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
