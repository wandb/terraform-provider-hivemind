package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wandb/terraform-provider-hivemind/internal/client"
)

var (
	_ resource.Resource                = (*environmentResource)(nil)
	_ resource.ResourceWithImportState = (*environmentResource)(nil)
)

func newEnvironmentResource() resource.Resource { return &environmentResource{} }

type environmentResource struct{ client *client.Client }

type environmentModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Repo              types.String `tfsdk:"repo"`
	Description       types.String `tfsdk:"description"`
	BaseImage         types.String `tfsdk:"base_image"`
	SetupScript       types.String `tfsdk:"setup_script"`
	WarmScript        types.String `tfsdk:"warm_script"`
	ValidateScript    types.String `tfsdk:"validate_script"`
	DefaultHarness    types.String `tfsdk:"default_harness"`
	Size              types.String `tfsdk:"size"`
	CPU               types.Int64  `tfsdk:"cpu"`
	MemoryGiB         types.Int64  `tfsdk:"memory_gib"`
	DiskGiB           types.Int64  `tfsdk:"disk_gib"`
	DockerEnabled     types.Bool   `tfsdk:"docker_enabled"`
	StartupMode       types.String `tfsdk:"startup_mode"`
	EgressAllowlist   types.List   `tfsdk:"egress_allowlist"`
	AutoDetectEgress  types.Bool   `tfsdk:"auto_detect_egress"`
	EnvSecrets        types.Map    `tfsdk:"env_secrets"`
	Integrations      types.List   `tfsdk:"integrations"`
	RebakeIntervalMin types.Int64  `tfsdk:"rebake_interval_min"`
	SandboxProvider   types.String `tfsdk:"sandbox_provider"`
	HardDelete        types.Bool   `tfsdk:"hard_delete"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

type environmentBody struct {
	RequestID         string            `json:"request_id,omitempty"`
	ExpectedUpdatedAt string            `json:"expected_updated_at,omitempty"`
	Name              string            `json:"name"`
	Repo              string            `json:"repo"`
	Description       string            `json:"description"`
	BaseImage         string            `json:"base_image"`
	SetupScript       string            `json:"setup_script"`
	WarmScript        *string           `json:"warm_script,omitempty"`
	ValidateScript    *string           `json:"validate_script,omitempty"`
	DefaultHarness    string            `json:"default_harness"`
	Size              *string           `json:"size,omitempty"`
	CPU               *int64            `json:"cpu,omitempty"`
	MemoryGiB         *int64            `json:"memory_gib,omitempty"`
	DiskGiB           *int64            `json:"disk_gib,omitempty"`
	DockerEnabled     bool              `json:"docker_enabled"`
	StartupMode       string            `json:"startup_mode"`
	EgressAllowlist   *[]string         `json:"egress_allowlist,omitempty"`
	AutoDetectEgress  *bool             `json:"auto_detect_egress,omitempty"`
	EnvSecrets        map[string]string `json:"env_secrets"`
	Integrations      []string          `json:"integrations,omitempty"`
	RebakeIntervalMin int64             `json:"rebake_interval_min"`
	SandboxProvider   *string           `json:"sandbox_provider,omitempty"`
}

type environmentResponse struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Repo              string            `json:"repo"`
	Description       string            `json:"description"`
	BaseImage         string            `json:"base_image"`
	SetupScript       string            `json:"setup_script"`
	WarmScript        *string           `json:"warm_script"`
	ValidateScript    *string           `json:"validate_script"`
	DefaultHarness    string            `json:"default_harness"`
	Size              string            `json:"size"`
	CPU               int64             `json:"cpu"`
	MemoryGiB         int64             `json:"memory_gib"`
	DiskGiB           int64             `json:"disk_gib"`
	DockerEnabled     bool              `json:"docker_enabled"`
	StartupMode       string            `json:"startup_mode"`
	EgressAllowlist   []string          `json:"egress_allowlist"`
	EnvSecrets        map[string]string `json:"env_secrets"`
	Integrations      []string          `json:"integrations"`
	RebakeIntervalMin int64             `json:"rebake_interval_min"`
	SandboxProvider   string            `json:"sandbox_provider"`
	ArchivedAt        *string           `json:"archived_at"`
	UpdatedAt         string            `json:"updated_at"`
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forager_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Optional: true, Computed: true, MarkdownDescription: desc,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}
	computedInt := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{
			Optional: true, Computed: true, MarkdownDescription: desc,
			PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
		}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Forager sandbox environment. Destroying it archives the environment unless `hard_delete` is true.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":         schema.StringAttribute{Required: true, MarkdownDescription: "Environment display name."},
			"base_image":   schema.StringAttribute{Required: true, MarkdownDescription: "Base image ID available in the HiveMind deployment."},
			"setup_script": schema.StringAttribute{Required: true, MarkdownDescription: "Shell script run when baking the environment. Stored in state; reference sandbox secrets instead of embedding credentials."},
			"repo": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "`owner/repo` cloned into the sandbox.",
			},
			"description":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Environment description. Defaults to an empty string."},
			"warm_script":     schema.StringAttribute{Optional: true, MarkdownDescription: "Shell script run when warming the environment. Stored in state."},
			"validate_script": schema.StringAttribute{Optional: true, MarkdownDescription: "Shell script used to validate the environment. Stored in state."},
			"default_harness": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("claude_code"),
				Validators:          []validator.String{stringvalidator.OneOf("claude_code", "codex")},
				MarkdownDescription: "Default agent harness: `claude_code` (default) or `codex`.",
			},
			"size":       computedString("Named size preset. Leave unset to use `cpu`/`memory_gib`/`disk_gib` or the server default."),
			"cpu":        computedInt("vCPUs (1-12)."),
			"memory_gib": computedInt("Memory in GiB (2-32)."),
			"disk_gib":   computedInt("Disk in GiB (25-128)."),
			"docker_enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Enable Docker in the sandbox. Defaults to false.",
			},
			"startup_mode": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("eager"),
				Validators:          []validator.String{stringvalidator.OneOf("eager", "on_demand")},
				MarkdownDescription: "Startup mode: `eager` (default) or `on_demand`.",
			},
			"egress_allowlist": schema.ListAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Allowed egress domains. Leave unset to keep whatever the server holds, " +
					"including domains merged in by `auto_detect_egress`.",
			},
			"auto_detect_egress": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Sparkbox only. Record DNS lookups during the next bake and merge them into " +
					"`egress_allowlist`. The server clears the flag after a successful bake; Terraform keeps the " +
					"configured value and only sends it on create or when it changes. Unset uses the server " +
					"default (on for Sparkbox environments created without an allowlist).",
			},
			"env_secrets": schema.MapAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
				MarkdownDescription: "Map of sandbox env var name to sandbox-provider secret name.",
			},
			"integrations": schema.ListAttribute{
				ElementType: types.StringType, Optional: true,
				MarkdownDescription: "Service integrations to attach. Null inherits the org's default integrations.",
			},
			"rebake_interval_min": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(1440),
				MarkdownDescription: "Interval between scheduled rebakes, in minutes. Defaults to 1440.",
			},
			"sandbox_provider": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Sandbox runtime name. Defaults to the organization default. Changing an explicitly configured runtime replaces the environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"hard_delete": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Delete instead of archive on destroy.",
			},
			"updated_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Server revision timestamp used to detect concurrent updates."},
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func environmentPath(id string) string { return "/v1/forager/environments/" + url.PathEscape(id) }

func knownString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueStringPointer()
}

func knownBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func knownInt(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueInt64Pointer()
}

// body builds a request from the plan. Sizing comes from config only: the server
// reports resolved sizing (including size "custom") that it won't accept back.
func (m environmentModel) body(ctx context.Context, config environmentModel) environmentBody {
	b := environmentBody{
		Name:              m.Name.ValueString(),
		Repo:              m.Repo.ValueString(),
		Description:       m.Description.ValueString(),
		BaseImage:         m.BaseImage.ValueString(),
		SetupScript:       m.SetupScript.ValueString(),
		WarmScript:        knownString(m.WarmScript),
		ValidateScript:    knownString(m.ValidateScript),
		DefaultHarness:    m.DefaultHarness.ValueString(),
		Size:              knownString(config.Size),
		CPU:               knownInt(config.CPU),
		MemoryGiB:         knownInt(config.MemoryGiB),
		DiskGiB:           knownInt(config.DiskGiB),
		DockerEnabled:     m.DockerEnabled.ValueBool(),
		StartupMode:       m.StartupMode.ValueString(),
		AutoDetectEgress:  knownBool(config.AutoDetectEgress),
		EnvSecrets:        mapStrings(ctx, m.EnvSecrets),
		RebakeIntervalMin: m.RebakeIntervalMin.ValueInt64(),
		SandboxProvider:   knownString(m.SandboxProvider),
	}
	if !config.EgressAllowlist.IsNull() && !config.EgressAllowlist.IsUnknown() {
		allow := listStrings(ctx, config.EgressAllowlist)
		if allow == nil {
			allow = []string{}
		}
		b.EgressAllowlist = &allow
	}
	if !m.Integrations.IsNull() && !m.Integrations.IsUnknown() {
		b.Integrations = listStrings(ctx, m.Integrations)
	}
	return b
}

func (m *environmentModel) apply(in *environmentResponse) {
	m.ID = types.StringValue(in.ID)
	m.Name = types.StringValue(in.Name)
	m.Repo = types.StringValue(in.Repo)
	m.Description = types.StringValue(in.Description)
	m.BaseImage = types.StringValue(in.BaseImage)
	m.SetupScript = types.StringValue(in.SetupScript)
	m.WarmScript = types.StringPointerValue(in.WarmScript)
	m.ValidateScript = types.StringPointerValue(in.ValidateScript)
	m.DefaultHarness = types.StringValue(in.DefaultHarness)
	m.Size = types.StringValue(in.Size)
	m.CPU = types.Int64Value(in.CPU)
	m.MemoryGiB = types.Int64Value(in.MemoryGiB)
	m.DiskGiB = types.Int64Value(in.DiskGiB)
	m.DockerEnabled = types.BoolValue(in.DockerEnabled)
	m.StartupMode = types.StringValue(in.StartupMode)
	m.EgressAllowlist = stringList(in.EgressAllowlist...)
	m.EnvSecrets = stringMap(in.EnvSecrets)
	// Null means "inherit org defaults"; keep it null rather than echoing the resolved list.
	if !m.Integrations.IsNull() && in.Integrations != nil {
		m.Integrations = stringList(in.Integrations...)
	}
	m.RebakeIntervalMin = types.Int64Value(in.RebakeIntervalMin)
	m.SandboxProvider = types.StringValue(in.SandboxProvider)
	m.UpdatedAt = types.StringValue(in.UpdatedAt)
	if m.HardDelete.IsNull() || m.HardDelete.IsUnknown() {
		m.HardDelete = types.BoolValue(false)
	}
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := plan.body(ctx, config)
	body.RequestID = uuid.NewString()
	var out environmentResponse
	if err := r.client.Do(ctx, http.MethodPost, "/v1/forager/environments", body, &out); err != nil {
		resp.Diagnostics.AddError("Creating environment", err.Error())
		return
	}
	plan.apply(&out)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out environmentResponse
	err := r.client.Do(ctx, http.MethodGet, environmentPath(state.ID.ValueString()), nil, &out)
	if errors.Is(err, client.ErrNotFound) || (err == nil && out.ArchivedAt != nil) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading environment", err.Error())
		return
	}
	state.apply(&out)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := plan.body(ctx, config)
	body.SandboxProvider = nil
	body.RequestID = uuid.NewString()
	body.ExpectedUpdatedAt = state.UpdatedAt.ValueString()
	// The server clears auto_detect_egress after a discovery bake, so only an edit re-requests it.
	if plan.AutoDetectEgress.Equal(state.AutoDetectEgress) {
		body.AutoDetectEgress = nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		resp.Diagnostics.AddError("Encoding environment", err.Error())
		return
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		resp.Diagnostics.AddError("Encoding environment", err.Error())
		return
	}
	// PATCH only changes fields that are present, so removals must be sent explicitly.
	if plan.WarmScript.IsNull() && !state.WarmScript.IsNull() {
		payload["warm_script"] = nil
	}
	if plan.ValidateScript.IsNull() && !state.ValidateScript.IsNull() {
		payload["validate_script"] = nil
	}
	if plan.Integrations.IsNull() && !state.Integrations.IsNull() {
		payload["inherit_integrations"] = true
	}
	var out environmentResponse
	if err := r.client.Do(ctx, http.MethodPatch, environmentPath(state.ID.ValueString()), payload, &out); err != nil {
		resp.Diagnostics.AddError("Updating environment", err.Error())
		return
	}
	plan.apply(&out)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// ModifyPlan marks unconfigured sizing fields unknown when configured sizing changes,
// since the server resolves the others (e.g. a new size changes cpu/memory_gib/disk_gib).
func (r *environmentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state, config environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	changed := (!config.Size.IsNull() && !config.Size.Equal(state.Size)) ||
		(!config.CPU.IsNull() && !config.CPU.Equal(state.CPU)) ||
		(!config.MemoryGiB.IsNull() && !config.MemoryGiB.Equal(state.MemoryGiB)) ||
		(!config.DiskGiB.IsNull() && !config.DiskGiB.Equal(state.DiskGiB))
	if !changed {
		return
	}
	if config.Size.IsNull() {
		plan.Size = types.StringUnknown()
	}
	if config.CPU.IsNull() {
		plan.CPU = types.Int64Unknown()
	}
	if config.MemoryGiB.IsNull() {
		plan.MemoryGiB = types.Int64Unknown()
	}
	if config.DiskGiB.IsNull() {
		plan.DiskGiB = types.Int64Unknown()
	}
	resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var err error
	if state.HardDelete.ValueBool() {
		err = r.client.Do(ctx, http.MethodDelete, environmentPath(state.ID.ValueString()), nil, nil)
	} else {
		err = r.client.Do(ctx, http.MethodPatch, environmentPath(state.ID.ValueString())+"/archive",
			map[string]bool{"archived": true}, nil)
	}
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Deleting environment", err.Error())
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
