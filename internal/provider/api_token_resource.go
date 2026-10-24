package provider

import (
	"context"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithConfigure = &apiTokenResource{}

func NewAPITokenResource() resource.Resource { return &apiTokenResource{} }

type apiTokenResource struct {
	client *technitium.Client
}

type apiTokenModel struct {
	ID           types.String `tfsdk:"id"`
	User         types.String `tfsdk:"user"`
	Name         types.String `tfsdk:"name"`
	Token        types.String `tfsdk:"token"`
	PartialToken types.String `tfsdk:"partial_token"`
}

func (r *apiTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_token"
}

func (r *apiTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *apiTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Creates a non-expiring API token for a user. The token has the user's permissions and is stored " +
			"in Terraform state, so protect the state. Tokens cannot be imported because the server only returns the secret once.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Partial token (session identifier).", PlanModifiers: keep},
			"user":          schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Username the token acts as."},
			"name":          schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Token name shown in the session list."},
			"token":         schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keep, Description: "The API token."},
			"partial_token": schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "Non-secret token prefix used to identify the session."},
		},
	}
}

func (r *apiTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiTokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	t, err := r.client.CreateAPIToken(ctx, plan.User.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating API token", err.Error())
		return
	}

	partial := t.Token
	if len(partial) > 16 {
		partial = partial[:16]
	}
	plan.Token = types.StringValue(t.Token)
	plan.PartialToken = types.StringValue(partial)
	plan.ID = plan.PartialToken
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *apiTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiTokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sessions, err := r.client.ListSessions(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing sessions", err.Error())
		return
	}
	for _, s := range sessions {
		if s.Type == "ApiToken" && s.PartialToken == state.PartialToken.ValueString() &&
			strings.EqualFold(s.Username, state.User.ValueString()) {
			return
		}
	}
	// Revoked or the user was deleted.
	resp.State.RemoveResource(ctx)
}

func (r *apiTokenResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected update", "All API token attributes force replacement.")
}

func (r *apiTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiTokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSession(ctx, state.PartialToken.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error revoking API token", err.Error())
	}
}
