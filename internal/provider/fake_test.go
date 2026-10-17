package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// fakeHiveMind is an in-memory stand-in for the HiveMind Forager endpoints.
type fakeHiveMind struct {
	mu           sync.Mutex
	integrations map[string]map[string]any
	headers      map[string]map[string]string
	secrets      map[string]map[string]string
	environments map[string]map[string]any
	revision     int
	envSeq       int
	puts         int
	patches      []map[string]any
}

func newFakeHiveMind(t *testing.T) (*fakeHiveMind, *httptest.Server) {
	f := &fakeHiveMind{
		integrations: map[string]map[string]any{},
		headers:      map[string]map[string]string{},
		secrets:      map[string]map[string]string{},
		environments: map[string]map[string]any{},
	}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeHiveMind) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test-token" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/forager/"), "/")
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch {
	case r.URL.Path == "/v1/forager/org/service-integrations" && r.Method == http.MethodGet:
		list := []map[string]any{}
		for _, v := range f.integrations {
			list = append(list, v)
		}
		reply(map[string]any{"integrations": list})
	case len(parts) == 3 && parts[0] == "org" && parts[1] == "service-integrations":
		name := parts[2]
		if r.Method == http.MethodDelete {
			delete(f.integrations, name)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		f.puts++
		f.revision++
		hdrs := map[string]string{}
		for k, v := range body["headers"].(map[string]any) {
			hdrs[k] = v.(string)
		}
		f.headers[name] = hdrs
		delete(body, "headers")
		body["id"] = "int-" + name
		body["name"] = name
		body["scope"] = "organization"
		body["revision"] = fmt.Sprint(f.revision)
		body["has_credentials"] = len(hdrs) > 0
		f.integrations[name] = body
		reply(map[string]any{"name": name, "revision": body["revision"]})
	case r.URL.Path == "/v1/forager/org/integration-teams":
		reply(map[string]any{"teams": []map[string]any{{"team": "wandb/infra"}, {"team": "wandb/ml"}}})
	case r.URL.Path == "/v1/forager/sandbox-providers":
		reply(map[string]any{"default": "modal", "providers": []map[string]any{
			{"name": "modal", "default": true, "selectable": true, "secret_store": true, "terminal": true, "preview": true, "restricted_egress": false},
			{"name": "sparkbox", "default": false, "selectable": true, "secret_store": false, "terminal": true, "preview": false, "restricted_egress": true},
		}})
	case len(parts) >= 3 && parts[0] == "sandbox-providers" && parts[2] == "secrets":
		provider := parts[1]
		if f.secrets[provider] == nil {
			f.secrets[provider] = map[string]string{}
		}
		switch {
		case len(parts) == 3:
			names := []string{}
			for n := range f.secrets[provider] {
				names = append(names, n)
			}
			reply(map[string]any{"names": names})
		case r.Method == http.MethodPut:
			f.secrets[provider][parts[3]] = body["value"].(string)
			w.WriteHeader(http.StatusNoContent)
		default:
			delete(f.secrets[provider], parts[3])
			w.WriteHeader(http.StatusNoContent)
		}
	case r.URL.Path == "/v1/forager/environments" && r.Method == http.MethodPost:
		f.envSeq++
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", f.envSeq)
		env := map[string]any{
			"id": id, "size": "small", "cpu": 2, "memory_gib": 4, "disk_gib": 25,
			"sandbox_provider": "modal", "archived_at": nil, "warm_script": nil, "validate_script": nil,
			"integrations": nil,
		}
		for k, v := range body {
			if k != "request_id" {
				env[k] = v
			}
		}
		env["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		f.environments[id] = env
		w.WriteHeader(http.StatusCreated)
		reply(env)
	case len(parts) >= 2 && parts[0] == "environments":
		env, ok := f.environments[parts[1]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		switch {
		case r.Method == http.MethodGet:
			reply(env)
		case r.Method == http.MethodDelete:
			delete(f.environments, parts[1])
			w.WriteHeader(http.StatusNoContent)
		case len(parts) == 3 && parts[2] == "archive":
			env["archived_at"] = time.Now().UTC().Format(time.RFC3339Nano)
			reply(env)
		default:
			f.patches = append(f.patches, body)
			if body["expected_updated_at"] != env["updated_at"] {
				http.Error(w, "stale", http.StatusConflict)
				return
			}
			for k, v := range body {
				if k != "request_id" && k != "expected_updated_at" && k != "inherit_integrations" {
					env[k] = v
				}
			}
			env["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
			reply(env)
		}
	default:
		http.NotFound(w, r)
	}
}

func testProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"hivemind": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func providerBlock(endpoint string) string {
	return fmt.Sprintf("provider \"hivemind\" {\n  endpoint = %q\n  token    = \"test-token\"\n}\n", endpoint)
}
