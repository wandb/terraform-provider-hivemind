package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wandb/terraform-provider-hivemind/internal/client"
)

var (
	_ resource.Resource                = (*sandboxSecretResource)(nil)
	_ resource.ResourceWithImportState = (*sandboxSecretResource)(nil)
)

func newSandboxSecretResource() resource.Resource { return &sandboxSecretResource{} }

type sandboxSecretResource struct{ client *client.Client }

type sandboxSecretModel struct {
	ID             types.String `tfsdk:"id"`
	Provider       types.String `tfsdk:"sandbox_provider"`
	Name           types.String `tfsdk:"name"`
	ValueWO        types.String `tfsdk:"value_wo"`
	ValueWOVersion types.String `tfsdk:"value_wo_version"`
}

func (r *sandboxSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forager_sandbox_secret"
}

func (r *sandboxSecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A secret stored in a sandbox provider's secret store, referenced by environments' `env_secrets`. " +
			"The value is write-only and never stored in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"sandbox_provider": schema.StringAttribute{Required: true, PlanModifiers: replace, MarkdownDescription: "Sandbox runtime with a secret store. Changing it replaces the secret."},
			"name":             schema.StringAttribute{Required: true, PlanModifiers: replace, MarkdownDescription: "Secret name referenced by environment `env_secrets`. Changing it replaces the secret."},
			"value_wo": schema.StringAttribute{
				Required: true, Sensitive: true, WriteOnly: true,
				MarkdownDescription: "Secret value. Write-only: sent on create and whenever `value_wo_version` changes.",
			},
			"value_wo_version": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Change to re-send `value_wo` (e.g. after rotation).",
			},
		},
	}
}

func (r *sandboxSecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func secretsPath(provider string) string {
	return "/v1/forager/sandbox-providers/" + url.PathEscape(provider) + "/secrets"
}

func (r *sandboxSecretResource) write(ctx context.Context, plan sandboxSecretModel, value string) error {
	p := secretsPath(plan.Provider.ValueString()) + "/" + url.PathEscape(plan.Name.ValueString())
	return r.client.Do(ctx, http.MethodPut, p, map[string]string{"value": value}, nil)
}

func (r *sandboxSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config sandboxSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, plan, config.ValueWO.ValueString()); err != nil {
		resp.Diagnostics.AddError("Writing sandbox secret", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.Provider.ValueString() + "/" + plan.Name.ValueString())
	plan.ValueWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sandboxSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sandboxSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out struct {
		Names []string `json:"names"`
	}
	err := r.client.Do(ctx, http.MethodGet, secretsPath(state.Provider.ValueString()), nil, &out)
	if errors.Is(err, client.ErrNotFound) || (err == nil && !slices.Contains(out.Names, state.Name.ValueString())) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading sandbox secrets", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *sandboxSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config sandboxSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, plan, config.ValueWO.ValueString()); err != nil {
		resp.Diagnostics.AddError("Writing sandbox secret", err.Error())
		return
	}
	plan.ValueWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sandboxSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sandboxSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := secretsPath(state.Provider.ValueString()) + "/" + url.PathEscape(state.Name.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, p, nil, nil); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Deleting sandbox secret", err.Error())
	}
}

func (r *sandboxSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	provider, name, ok := strings.Cut(req.ID, "/")
	if !ok || provider == "" || name == "" {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <sandbox_provider>/<name>, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("sandbox_provider"), provider)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}
