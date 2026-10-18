// Package provider implements the HiveMind Terraform provider.
package provider

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wandb/terraform-provider-hivemind/internal/client"
)

const defaultEndpoint = "https://hivemind.wandb.tools"

var _ provider.Provider = (*hivemindProvider)(nil)

// New returns a provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &hivemindProvider{version: version} }
}

type hivemindProvider struct{ version string }

type providerModel struct {
	Endpoint     types.String `tfsdk:"endpoint"`
	OrgID        types.String `tfsdk:"org_id"`
	Token        types.String `tfsdk:"token"`
	OIDCToken    types.String `tfsdk:"oidc_token"`
	OIDCAudience types.String `tfsdk:"oidc_audience"`
}

func (p *hivemindProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "hivemind"
	resp.Version = p.version
}

func (p *hivemindProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage HiveMind Forager resources with a service account that has the " +
			"`read` and `manage` scopes. Authenticate with a static service-account token, an OIDC " +
			"id_token, or (inside GitHub Actions) the runner's OIDC token, which is exchanged at " +
			"`/v1/auth/actions/exchange` for a short-lived HiveMind token.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "HiveMind API URL. Defaults to `$HIVEMIND_ENDPOINT` or `" + defaultEndpoint + "`.",
			},
			"org_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Organization to act in (sent as `X-Org-Id`). Defaults to `$HIVEMIND_ORG_ID`; usually unnecessary for service accounts.",
			},
			"token": schema.StringAttribute{
				Optional: true, Sensitive: true,
				MarkdownDescription: "Service-account token. Defaults to `$HIVEMIND_TOKEN`.",
			},
			"oidc_token": schema.StringAttribute{
				Optional: true, Sensitive: true,
				MarkdownDescription: "OIDC id_token to exchange for a HiveMind token. Defaults to `$HIVEMIND_OIDC_TOKEN`.",
			},
			"oidc_audience": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Audience requested from GitHub Actions. Defaults to the endpoint, matching the HiveMind daemon.",
			},
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (p *hivemindProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpoint := strings.TrimRight(firstNonEmpty(cfg.Endpoint.ValueString(), os.Getenv("HIVEMIND_ENDPOINT"), defaultEndpoint), "/")
	token := firstNonEmpty(cfg.Token.ValueString(), os.Getenv("HIVEMIND_TOKEN"))
	oidcToken := firstNonEmpty(cfg.OIDCToken.ValueString(), os.Getenv("HIVEMIND_OIDC_TOKEN"))

	c := client.Config{
		Endpoint:  endpoint,
		OrgID:     firstNonEmpty(cfg.OrgID.ValueString(), os.Getenv("HIVEMIND_ORG_ID")),
		UserAgent: "terraform-provider-hivemind/" + p.version,
	}
	switch {
	case token != "" && oidcToken != "":
		resp.Diagnostics.AddAttributeError(path.Root("token"), "Conflicting credentials", "Set only one of token or oidc_token.")
		return
	case token != "":
		c.Token = token
	case oidcToken != "":
		c.OIDCToken = client.StaticOIDC(oidcToken)
	case os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL") != "" && os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN") != "":
		c.OIDCToken = client.GitHubActionsOIDC(
			os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"),
			os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"),
			firstNonEmpty(cfg.OIDCAudience.ValueString(), endpoint),
			&http.Client{Timeout: 30 * time.Second},
		)
	default:
		resp.Diagnostics.AddError("Missing credentials",
			"Set token / HIVEMIND_TOKEN, oidc_token / HIVEMIND_OIDC_TOKEN, or run in GitHub Actions with `id-token: write`.")
		return
	}
	cl, err := client.New(c)
	if err != nil {
		resp.Diagnostics.AddError("Configuring HiveMind client", err.Error())
		return
	}
	resp.ResourceData = cl
	resp.DataSourceData = cl
}

func (p *hivemindProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newOrgServiceIntegrationResource,
		newSandboxSecretResource,
		newEnvironmentResource,
	}
}

func (p *hivemindProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newIntegrationTeamsDataSource,
		newSandboxProvidersDataSource,
	}
}
