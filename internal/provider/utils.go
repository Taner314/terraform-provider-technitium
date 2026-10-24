package provider

import (
	"fmt"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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
