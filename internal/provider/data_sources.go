package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wandb/terraform-provider-hivemind/internal/client"
)

func newIntegrationTeamsDataSource() datasource.DataSource { return &integrationTeamsDataSource{} }

type integrationTeamsDataSource struct{ client *client.Client }

func (d *integrationTeamsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forager_integration_teams"
}

func (d *integrationTeamsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Team slugs accepted in `allowed_teams` of organization service integrations.",
		Attributes: map[string]schema.Attribute{
			"teams": schema.ListAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Available team slugs in `owner/team` format."},
		},
	}
}

func (d *integrationTeamsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *integrationTeamsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var out struct {
		Teams []struct {
			Team string `json:"team"`
		} `json:"teams"`
	}
	if err := d.client.Do(ctx, http.MethodGet, "/v1/forager/org/integration-teams", nil, &out); err != nil {
		resp.Diagnostics.AddError("Listing integration teams", err.Error())
		return
	}
	teams := make([]string, 0, len(out.Teams))
	for _, t := range out.Teams {
		teams = append(teams, t.Team)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("teams"), stringList(teams...))...)
}

func newSandboxProvidersDataSource() datasource.DataSource { return &sandboxProvidersDataSource{} }

type sandboxProvidersDataSource struct{ client *client.Client }

type sandboxProviderModel struct {
	Name        types.String `tfsdk:"name"`
	Default     types.Bool   `tfsdk:"default"`
	Selectable  types.Bool   `tfsdk:"selectable"`
	SecretStore types.Bool   `tfsdk:"secret_store"`
}

type sandboxProvidersModel struct {
	Default   types.String           `tfsdk:"default"`
	Providers []sandboxProviderModel `tfsdk:"providers"`
}

func (d *sandboxProvidersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forager_sandbox_providers"
}

func (d *sandboxProvidersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sandbox runtimes enabled for the organization.",
		Attributes: map[string]schema.Attribute{
			"default": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization default runtime name."},
			"providers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Runtimes enabled for the organization and their capabilities.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":         schema.StringAttribute{Computed: true, MarkdownDescription: "Runtime name used in `sandbox_provider`."},
					"default":      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this is the organization default runtime."},
					"selectable":   schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether users can select this runtime."},
					"secret_store": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this runtime supports sandbox secrets."},
				}},
			},
		},
	}
}

func (d *sandboxProvidersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *sandboxProvidersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var out struct {
		Default   string `json:"default"`
		Providers []struct {
			Name        string `json:"name"`
			Default     bool   `json:"default"`
			Selectable  bool   `json:"selectable"`
			SecretStore bool   `json:"secret_store"`
		} `json:"providers"`
	}
	if err := d.client.Do(ctx, http.MethodGet, "/v1/forager/sandbox-providers", nil, &out); err != nil {
		resp.Diagnostics.AddError("Listing sandbox providers", err.Error())
		return
	}
	state := sandboxProvidersModel{Default: types.StringValue(out.Default), Providers: []sandboxProviderModel{}}
	for _, p := range out.Providers {
		state.Providers = append(state.Providers, sandboxProviderModel{
			Name: types.StringValue(p.Name), Default: types.BoolValue(p.Default),
			Selectable: types.BoolValue(p.Selectable), SecretStore: types.BoolValue(p.SecretStore),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func newBaseImagesDataSource() datasource.DataSource { return &baseImagesDataSource{} }

type baseImagesDataSource struct{ client *client.Client }

type baseImageModel struct {
	ImageRef  types.String `tfsdk:"image_ref"`
	Name      types.String `tfsdk:"name"`
	Version   types.String `tfsdk:"version"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type baseImagesModel struct {
	Images []baseImageModel `tfsdk:"images"`
}

func (d *baseImagesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forager_base_images"
}

func (d *baseImagesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Base images in the deployment's catalog, accepted in `base_image` of environments.",
		Attributes: map[string]schema.Attribute{
			"images": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Catalog images in server order.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"image_ref":  schema.StringAttribute{Computed: true, MarkdownDescription: "Image ID used in `base_image`, e.g. `sparkbox-base://universal`."},
					"name":       schema.StringAttribute{Computed: true, MarkdownDescription: "Display name, e.g. `Ubuntu`."},
					"version":    schema.StringAttribute{Computed: true, MarkdownDescription: "Image version or build identifier."},
					"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 build timestamp. Null for built-in images."},
				}},
			},
		},
	}
}

func (d *baseImagesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *baseImagesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var out []struct {
		ImageRef  string  `json:"image_ref"`
		Name      string  `json:"name"`
		Version   string  `json:"version"`
		CreatedAt *string `json:"created_at"`
	}
	if err := d.client.Do(ctx, http.MethodGet, "/v1/forager/base-images", nil, &out); err != nil {
		resp.Diagnostics.AddError("Listing base images", err.Error())
		return
	}
	state := baseImagesModel{Images: []baseImageModel{}}
	for _, img := range out {
		state.Images = append(state.Images, baseImageModel{
			ImageRef: types.StringValue(img.ImageRef), Name: types.StringValue(img.Name),
			Version: types.StringValue(img.Version), CreatedAt: types.StringPointerValue(img.CreatedAt),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
