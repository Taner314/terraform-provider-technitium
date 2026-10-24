package provider

import (
	"context"
	"fmt"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func convertStringListToTF(items []string) []types.String {

	if len(items) == 0 {
		return nil
	}

	var attr []types.String
	for _, item := range items {
		attr = append(attr, types.StringValue(item))
	}

	return attr

}

func ConfigureResourceClient(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *technitium.Client {
	if req.ProviderData == nil {
		return nil
	}

	client, ok := req.ProviderData.(*technitium.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Configure Type",
			fmt.Sprintf("Expected *technitium.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return nil
	}

	return client
}

func ConfigureDataSourceClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *technitium.Client {
	if req.ProviderData == nil {
		return nil
	}

	client, ok := req.ProviderData.(*technitium.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Configure Type",
			fmt.Sprintf("Expected *technitium.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return nil
	}

	return client
}

// copyFrom plans a computed id as the value of another string attribute, so
// renames show the new id instead of keeping the old one.
type copyFromModifier struct{ from path.Path }

func copyFrom(attr string) planmodifier.String { return copyFromModifier{from: path.Root(attr)} }

func (m copyFromModifier) Description(context.Context) string {
	return "Set to the value of " + m.from.String() + "."
}

func (m copyFromModifier) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (m copyFromModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	var v types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, m.from, &v)...)
	if known(v) {
		resp.PlanValue = v
	}
}
