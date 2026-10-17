package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wandb/terraform-provider-hivemind/internal/client"
)

func configuredClient(data any, diags *diag.Diagnostics) *client.Client {
	if data == nil {
		return nil
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", data))
	}
	return c
}

func stringValues(values []string) []attr.Value {
	out := make([]attr.Value, 0, len(values))
	for _, v := range values {
		out = append(out, types.StringValue(v))
	}
	return out
}

func stringList(values ...string) types.List {
	return types.ListValueMust(types.StringType, stringValues(values))
}

func stringSet(values []string) types.Set {
	return types.SetValueMust(types.StringType, stringValues(values))
}

func stringMap(values map[string]string) types.Map {
	out := make(map[string]attr.Value, len(values))
	for k, v := range values {
		out[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, out)
}

func listStrings(ctx context.Context, v types.List) []string {
	out := []string{}
	if v.IsNull() || v.IsUnknown() {
		return out
	}
	v.ElementsAs(ctx, &out, false)
	return out
}

func setStrings(ctx context.Context, v types.Set) []string {
	out := []string{}
	if v.IsNull() || v.IsUnknown() {
		return out
	}
	v.ElementsAs(ctx, &out, false)
	return out
}

func mapStrings(ctx context.Context, v types.Map) map[string]string {
	out := map[string]string{}
	if v.IsNull() || v.IsUnknown() {
		return out
	}
	v.ElementsAs(ctx, &out, false)
	return out
}

func emptyToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}
