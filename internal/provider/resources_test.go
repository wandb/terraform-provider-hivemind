package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func noSecretInState(secret string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			for k, v := range rs.Primary.Attributes {
				if strings.Contains(v, secret) {
					return fmt.Errorf("%s.%s contains a secret value", name, k)
				}
			}
		}
		return nil
	}
}

func integrationConfig(endpoint, token, version string) string {
	return providerBlock(endpoint) + fmt.Sprintf(`
resource "hivemind_forager_org_service_integration" "datadog" {
  name    = "datadog"
  target  = "https://api.us5.datadoghq.com"
  methods = ["GET", "HEAD", "POST"]
  env = {
    DD_SITE         = "{proxy_host}"
    DD_ACCESS_TOKEN = "forager-proxy"
    PUP_TRUST_SITE  = "1"
  }
  allowed_teams      = ["wandb/infra"]
  headers_wo         = { Authorization = "Bearer %s" }
  headers_wo_version = %q
}
`, token, version)
}

func TestOrgServiceIntegrationResource(t *testing.T) {
	f, srv := newFakeHiveMind(t)
	const name = "hivemind_forager_org_service_integration.datadog"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: integrationConfig(srv.URL, "dd-secret-1", "1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "int-datadog"),
					resource.TestCheckResourceAttr(name, "kind", "http"),
					resource.TestCheckResourceAttr(name, "has_credentials", "true"),
					resource.TestCheckResourceAttr(name, "paths.0", "/"),
					resource.TestCheckResourceAttr(name, "env.DD_SITE", "{proxy_host}"),
					resource.TestCheckNoResourceAttr(name, "headers_wo"),
					noSecretInState("dd-secret-1"),
					func(*terraform.State) error {
						if got := f.headers["datadog"]["Authorization"]; got != "Bearer dd-secret-1" {
							return fmt.Errorf("server header = %q", got)
						}
						return nil
					},
				),
			},
			{
				// Bumping the version re-sends the rotated header.
				Config: integrationConfig(srv.URL, "dd-secret-2", "2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					noSecretInState("dd-secret-2"),
					func(*terraform.State) error {
						if got := f.headers["datadog"]["Authorization"]; got != "Bearer dd-secret-2" {
							return fmt.Errorf("server header = %q", got)
						}
						return nil
					},
				),
			},
			{
				ResourceName:                         name,
				ImportState:                          true,
				ImportStateId:                        "datadog",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"headers_wo_version", "revision"},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if _, ok := f.integrations["datadog"]; ok {
				return fmt.Errorf("integration still exists")
			}
			return nil
		},
	})
}

func secretConfig(endpoint, value, version string) string {
	return providerBlock(endpoint) + fmt.Sprintf(`
resource "hivemind_forager_sandbox_secret" "gh" {
  sandbox_provider = "modal"
  name             = "GH_TOKEN"
  value_wo         = %q
  value_wo_version = %q
}
`, value, version)
}

func TestSandboxSecretResource(t *testing.T) {
	f, srv := newFakeHiveMind(t)
	const name = "hivemind_forager_sandbox_secret.gh"
	serverValue := func(want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			if got := f.secrets["modal"]["GH_TOKEN"]; got != want {
				return fmt.Errorf("server value = %q, want %q", got, want)
			}
			return nil
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: secretConfig(srv.URL, "s3cr3t-a", "1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "modal/GH_TOKEN"),
					resource.TestCheckNoResourceAttr(name, "value_wo"),
					noSecretInState("s3cr3t-a"),
					serverValue("s3cr3t-a"),
				),
			},
			{
				// A changed value without a version bump isn't sent.
				Config:   secretConfig(srv.URL, "s3cr3t-b", "1"),
				PlanOnly: true,
			},
			{
				Config: secretConfig(srv.URL, "s3cr3t-b", "2"),
				Check:  resource.ComposeAggregateTestCheckFunc(noSecretInState("s3cr3t-b"), serverValue("s3cr3t-b")),
			},
			{
				ResourceName:            name,
				ImportState:             true,
				ImportStateId:           "modal/GH_TOKEN",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value_wo_version"},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if _, ok := f.secrets["modal"]["GH_TOKEN"]; ok {
				return fmt.Errorf("secret still exists")
			}
			return nil
		},
	})
}

func environmentConfig(endpoint, script string, hardDelete bool) string {
	return providerBlock(endpoint) + fmt.Sprintf(`
resource "hivemind_forager_environment" "test" {
  name         = "agentstream"
  repo         = "wandb/agentstream"
  base_image   = "python:3.12"
  setup_script = %q
  env_secrets  = { GH_TOKEN = "GH_TOKEN" }
  integrations = ["datadog"]
  hard_delete  = %t
}
`, script, hardDelete)
}

func TestEnvironmentResourceArchivesByDefault(t *testing.T) {
	f, srv := newFakeHiveMind(t)
	const name = "hivemind_forager_environment.test"
	var id string
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: environmentConfig(srv.URL, "make setup", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "size", "small"),
					resource.TestCheckResourceAttr(name, "cpu", "2"),
					resource.TestCheckResourceAttr(name, "sandbox_provider", "modal"),
					resource.TestCheckResourceAttr(name, "integrations.0", "datadog"),
					resource.TestCheckResourceAttrWith(name, "id", func(v string) error { id = v; return nil }),
				),
			},
			{
				Config: environmentConfig(srv.URL, "make setup && make test", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "setup_script", "make setup && make test"),
					func(*terraform.State) error {
						if len(f.patches) != 1 {
							return fmt.Errorf("patches = %d", len(f.patches))
						}
						p := f.patches[0]
						if p["expected_updated_at"] == nil || p["sandbox_provider"] != nil || p["size"] != nil {
							return fmt.Errorf("unexpected patch body %v", p)
						}
						return nil
					},
				),
			},
		},
		CheckDestroy: func(*terraform.State) error {
			env, ok := f.environments[id]
			if !ok || env["archived_at"] == nil {
				return fmt.Errorf("environment %s should be archived, not deleted", id)
			}
			return nil
		},
	})
}

func TestEnvironmentResourceHardDelete(t *testing.T) {
	f, srv := newFakeHiveMind(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps:                    []resource.TestStep{{Config: environmentConfig(srv.URL, "make setup", true)}},
		CheckDestroy: func(*terraform.State) error {
			if len(f.environments) != 0 {
				return fmt.Errorf("environment should be deleted")
			}
			return nil
		},
	})
}

func TestEnvironmentResourceUpdateClearsAndResizes(t *testing.T) {
	f, srv := newFakeHiveMind(t)
	const name = "hivemind_forager_environment.test"
	base := providerBlock(srv.URL) + `
resource "hivemind_forager_environment" "test" {
  name         = "agentstream"
  base_image   = "python:3.12"
  setup_script = "make setup"
%s}
`
	var id string
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(base, "  warm_script = \"make warm\"\n  integrations = [\"datadog\"]\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "warm_script", "make warm"),
					resource.TestCheckResourceAttr(name, "cpu", "2"),
					resource.TestCheckResourceAttrWith(name, "id", func(v string) error { id = v; return nil }),
				),
			},
			{
				Config: fmt.Sprintf(base, "  size = \"medium\"\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(name, "warm_script"),
					resource.TestCheckNoResourceAttr(name, "integrations"),
					resource.TestCheckResourceAttr(name, "cpu", "4"),
					resource.TestCheckResourceAttr(name, "memory_gib", "8"),
					func(*terraform.State) error {
						p := f.patches[len(f.patches)-1]
						if v, ok := p["warm_script"]; !ok || v != nil {
							return fmt.Errorf("removed warm_script should be sent as null: %v", p)
						}
						if p["inherit_integrations"] != true {
							return fmt.Errorf("removed integrations should send inherit_integrations: %v", p)
						}
						if f.environments[id]["integrations"] != nil {
							return fmt.Errorf("integrations should be inherited")
						}
						return nil
					},
				),
			},
		},
	})
}

func TestEnvironmentResourceAutoDetectEgress(t *testing.T) {
	f, srv := newFakeHiveMind(t)
	const name = "hivemind_forager_environment.test"
	config := providerBlock(srv.URL) + `
resource "hivemind_forager_environment" "test" {
  name               = "core"
  repo               = "wandb/core"
  base_image         = "python:3.12"
  setup_script       = "make setup"
  sandbox_provider   = "sparkbox"
  auto_detect_egress = true
}
`
	var id string
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "auto_detect_egress", "true"),
					resource.TestCheckResourceAttr(name, "egress_allowlist.#", "0"),
					resource.TestCheckResourceAttrWith(name, "id", func(v string) error { id = v; return nil }),
					func(*terraform.State) error {
						c := f.creates[0]
						if c["auto_detect_egress"] != true {
							return fmt.Errorf("create body missing auto_detect_egress: %v", c)
						}
						if _, ok := c["egress_allowlist"]; ok {
							return fmt.Errorf("unset egress_allowlist should not be sent: %v", c)
						}
						return nil
					},
				),
			},
			{
				// After discovery the server merges domains and clears the flag; neither is drift.
				PreConfig: func() { f.completeDiscovery(id, "pypi.org") },
				Config:    config,
				PlanOnly:  true,
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "egress_allowlist.0", "pypi.org"),
					resource.TestCheckResourceAttr(name, "auto_detect_egress", "true"),
					func(*terraform.State) error {
						if len(f.patches) != 0 {
							return fmt.Errorf("expected no PATCH, got %v", f.patches)
						}
						return nil
					},
				),
			},
			{
				Config: strings.Replace(config, "auto_detect_egress = true", "auto_detect_egress = false", 1),
				Check: func(*terraform.State) error {
					if len(f.patches) != 1 || f.patches[0]["auto_detect_egress"] != false {
						return fmt.Errorf("toggling auto_detect_egress should PATCH it: %v", f.patches)
					}
					if _, ok := f.patches[0]["egress_allowlist"]; ok {
						return fmt.Errorf("unset egress_allowlist should not be sent: %v", f.patches[0])
					}
					return nil
				},
			},
		},
	})
}

func TestDataSources(t *testing.T) {
	_, srv := newFakeHiveMind(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(),
		Steps: []resource.TestStep{{
			Config: providerBlock(srv.URL) + `
data "hivemind_forager_integration_teams" "all" {}
data "hivemind_forager_sandbox_providers" "all" {}
data "hivemind_forager_base_images" "all" {}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.hivemind_forager_integration_teams.all", "teams.#", "2"),
				resource.TestCheckResourceAttr("data.hivemind_forager_integration_teams.all", "teams.0", "wandb/infra"),
				resource.TestCheckResourceAttr("data.hivemind_forager_sandbox_providers.all", "default", "modal"),
				resource.TestCheckResourceAttr("data.hivemind_forager_sandbox_providers.all", "providers.1.secret_store", "false"),
				resource.TestCheckResourceAttr("data.hivemind_forager_base_images.all", "images.#", "2"),
				resource.TestCheckResourceAttr("data.hivemind_forager_base_images.all", "images.0.image_ref", "sparkbox-base://universal"),
				resource.TestCheckNoResourceAttr("data.hivemind_forager_base_images.all", "images.0.created_at"),
				resource.TestCheckResourceAttr("data.hivemind_forager_base_images.all", "images.1.created_at", "2026-10-19T04:59:32Z"),
			),
		}},
	})
}
