package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wandb/terraform-provider-hivemind/internal/client"
)

var (
	_ resource.Resource                = (*orgServiceIntegrationResource)(nil)
	_ resource.ResourceWithImportState = (*orgServiceIntegrationResource)(nil)
)

func newOrgServiceIntegrationResource() resource.Resource { return &orgServiceIntegrationResource{} }

type orgServiceIntegrationResource struct{ client *client.Client }

type orgServiceIntegrationModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Kind             types.String `tfsdk:"kind"`
	Target           types.String `tfsdk:"target"`
	Audience         types.String `tfsdk:"audience"`
	WIFProvider      types.String `tfsdk:"wif_provider"`
	ServiceAccount   types.String `tfsdk:"service_account"`
	Paths            types.List   `tfsdk:"paths"`
	Methods          types.List   `tfsdk:"methods"`
	Env              types.Map    `tfsdk:"env"`
	Default          types.Bool   `tfsdk:"default"`
	AllowedTeams     types.Set    `tfsdk:"allowed_teams"`
	HeadersWO        types.Map    `tfsdk:"headers_wo"`
	HeadersWOVersion types.String `tfsdk:"headers_wo_version"`
	Revision         types.String `tfsdk:"revision"`
	HasCredentials   types.Bool   `tfsdk:"has_credentials"`
}

type serviceIntegrationRequest struct {
	Kind           string            `json:"kind"`
	Target         string            `json:"target"`
	Audience       *string           `json:"audience,omitempty"`
	WIFProvider    *string           `json:"wif_provider,omitempty"`
	ServiceAccount *string           `json:"service_account,omitempty"`
	Headers        map[string]string `json:"headers"`
	Paths          []string          `json:"paths"`
	Methods        []string          `json:"methods"`
	Env            map[string]string `json:"env"`
	Default        bool              `json:"default"`
	AllowedTeams   []string          `json:"allowed_teams"`
}

type serviceIntegration struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Kind           string            `json:"kind"`
	Target         string            `json:"target"`
	Audience       *string           `json:"audience"`
	WIFProvider    *string           `json:"wif_provider"`
	ServiceAccount *string           `json:"service_account"`
	Paths          []string          `json:"paths"`
	Methods        []string          `json:"methods"`
	Env            map[string]string `json:"env"`
	Default        bool              `json:"default"`
	AllowedTeams   []string          `json:"allowed_teams"`
	Revision       string            `json:"revision"`
	HasCredentials bool              `json:"has_credentials"`
}

func (r *orgServiceIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forager_org_service_integration"
}

func (r *orgServiceIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	nonEmpty := []validator.String{stringvalidator.LengthAtLeast(1)}
	resp.Schema = schema.Schema{
		MarkdownDescription: "An organization-wide Forager service integration. Forager's credential-injecting " +
			"proxy adds `headers_wo` to requests sent to `target`, so sandboxes never see the real credentials. " +
			"Requires a service account with the `manage` scope (and `read` for refresh).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Integration name; also the proxy hostname label (e.g. `datadog`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"kind": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("http"),
				Validators: []validator.String{stringvalidator.OneOf("http", "oidc")},
			},
			"target": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Upstream base URL, e.g. `https://api.us5.datadoghq.com`.",
			},
			"audience":        schema.StringAttribute{Optional: true, Validators: nonEmpty},
			"wif_provider":    schema.StringAttribute{Optional: true, Validators: nonEmpty},
			"service_account": schema.StringAttribute{Optional: true, Validators: nonEmpty},
			"paths": schema.ListAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				Default: listdefault.StaticValue(stringList("/")),
			},
			"methods": schema.ListAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				Default: listdefault.StaticValue(stringList("GET", "HEAD")),
			},
			"env": schema.MapAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
				MarkdownDescription: "Environment variables set in sandboxes, e.g. `DD_SITE = \"{proxy_host}\"`.",
			},
			"default": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Attach to environments that don't list integrations explicitly.",
			},
			"allowed_teams": schema.SetAttribute{
				ElementType: types.StringType, Optional: true,
				MarkdownDescription: "`owner/team` slugs allowed to use the integration. Null grants every org member.",
			},
			"headers_wo": schema.MapAttribute{
				ElementType: types.StringType, Optional: true, Sensitive: true, WriteOnly: true,
				MarkdownDescription: "Headers the proxy injects. Write-only: never stored in state. " +
					"Every apply that updates this resource re-sends them; omitting them removes stored credentials.",
			},
			"headers_wo_version": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Change to force an update that re-sends `headers_wo` (e.g. after rotating a token).",
			},
			"revision":        schema.StringAttribute{Computed: true},
			"has_credentials": schema.BoolAttribute{Computed: true},
		},
	}
}

func (r *orgServiceIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func integrationPath(name string) string {
	return "/v1/forager/org/service-integrations/" + url.PathEscape(name)
}

func (r *orgServiceIntegrationResource) put(ctx context.Context, plan, config orgServiceIntegrationModel) (*serviceIntegration, error) {
	body := serviceIntegrationRequest{
		Kind:           plan.Kind.ValueString(),
		Target:         plan.Target.ValueString(),
		Audience:       plan.Audience.ValueStringPointer(),
		WIFProvider:    plan.WIFProvider.ValueStringPointer(),
		ServiceAccount: plan.ServiceAccount.ValueStringPointer(),
		Headers:        map[string]string{},
		Paths:          listStrings(ctx, plan.Paths),
		Methods:        listStrings(ctx, plan.Methods),
		Env:            mapStrings(ctx, plan.Env),
		Default:        plan.Default.ValueBool(),
	}
	if !plan.AllowedTeams.IsNull() {
		body.AllowedTeams = setStrings(ctx, plan.AllowedTeams)
	}
	if !config.HeadersWO.IsNull() {
		body.Headers = mapStrings(ctx, config.HeadersWO)
	}
	if err := r.client.Do(ctx, http.MethodPut, integrationPath(plan.Name.ValueString()), body, nil); err != nil {
		return nil, err
	}
	return r.find(ctx, plan.Name.ValueString())
}

func (r *orgServiceIntegrationResource) find(ctx context.Context, name string) (*serviceIntegration, error) {
	var out struct {
		Integrations []serviceIntegration `json:"integrations"`
	}
	if err := r.client.Do(ctx, http.MethodGet, "/v1/forager/org/service-integrations", nil, &out); err != nil {
		return nil, err
	}
	for i := range out.Integrations {
		if out.Integrations[i].Name == name {
			return &out.Integrations[i], nil
		}
	}
	return nil, client.ErrNotFound
}

func (m *orgServiceIntegrationModel) apply(ctx context.Context, in *serviceIntegration) {
	m.ID = types.StringValue(in.ID)
	m.Name = types.StringValue(in.Name)
	m.Kind = types.StringValue(in.Kind)
	m.Target = types.StringValue(in.Target)
	m.Audience = types.StringPointerValue(emptyToNil(in.Audience))
	m.WIFProvider = types.StringPointerValue(emptyToNil(in.WIFProvider))
	m.ServiceAccount = types.StringPointerValue(emptyToNil(in.ServiceAccount))
	m.Paths = stringList(in.Paths...)
	m.Methods = stringList(in.Methods...)
	m.Env = stringMap(in.Env)
	m.Default = types.BoolValue(in.Default)
	if in.AllowedTeams == nil {
		m.AllowedTeams = types.SetNull(types.StringType)
	} else {
		m.AllowedTeams = stringSet(in.AllowedTeams)
	}
	m.HeadersWO = types.MapNull(types.StringType)
	m.Revision = types.StringValue(in.Revision)
	m.HasCredentials = types.BoolValue(in.HasCredentials)
}

func (r *orgServiceIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config orgServiceIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.put(ctx, plan, config)
	if err != nil {
		resp.Diagnostics.AddError("Creating service integration", err.Error())
		return
	}
	plan.apply(ctx, got)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *orgServiceIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgServiceIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.find(ctx, state.Name.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading service integration", err.Error())
		return
	}
	state.apply(ctx, got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *orgServiceIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config orgServiceIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.put(ctx, plan, config)
	if err != nil {
		resp.Diagnostics.AddError("Updating service integration", err.Error())
		return
	}
	plan.apply(ctx, got)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *orgServiceIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgServiceIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, http.MethodDelete, integrationPath(state.Name.ValueString()), nil, nil)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Deleting service integration", fmt.Sprintf("%s: %s", state.Name.ValueString(), err))
	}
}

func (r *orgServiceIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
