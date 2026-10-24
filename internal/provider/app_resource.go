package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &appResource{}
	_ resource.ResourceWithImportState = &appResource{}
)

func NewAppResource() resource.Resource { return &appResource{} }

type appResource struct {
	client *technitium.Client
}

type appModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	URL        types.String `tfsdk:"url"`
	Config     types.String `tfsdk:"config"`
	Version    types.String `tfsdk:"version"`
	ClassPaths types.List   `tfsdk:"class_paths"`
}

func (r *appResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *appResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *appResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Installs a DNS app and manages its config. Use `class_paths` in `technitium_dns_record_app`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "App name; also the import ID.", PlanModifiers: keep},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "App name, e.g. `Split Horizon`. Without `url` the app is installed from the DNS App Store by this name."},
			"url": schema.StringAttribute{Optional: true,
				Description: "`https://` URL of the app zip. Changing it updates the app in place. Omit to use the store's current version."},
			"config": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: keep,
				Description: "App config, usually JSON (`jsonencode(...)`). Compared as JSON, so formatting differences don't show as changes. Unset keeps the app's own config."},
			"version": schema.StringAttribute{Computed: true, Description: "Installed version.", PlanModifiers: keep},
			"class_paths": schema.ListAttribute{Computed: true, ElementType: types.StringType,
				Description:   "Class paths of the app's handlers, e.g. `SplitHorizon.SimpleAddress`.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
		},
	}
}

// sameConfig compares configs as JSON when both parse, else as text.
func sameConfig(a, b string) bool {
	var av, bv any
	if json.Unmarshal([]byte(a), &av) == nil && json.Unmarshal([]byte(b), &bv) == nil {
		return reflect.DeepEqual(av, bv)
	}
	return bytes.Equal(bytes.TrimSpace([]byte(a)), bytes.TrimSpace([]byte(b)))
}

func (r *appResource) install(ctx context.Context, m appModel, update bool) error {
	u := m.URL.ValueString()
	if u == "" {
		var err error
		if u, err = r.client.StoreAppURL(ctx, m.Name.ValueString()); err != nil {
			return err
		}
	}
	return r.client.DownloadApp(ctx, m.Name.ValueString(), u, update)
}

func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.install(ctx, plan, false); err != nil {
		resp.Diagnostics.AddError("Error installing app", err.Error())
		return
	}
	if known(plan.Config) {
		if err := r.client.SetAppConfig(ctx, plan.Name.ValueString(), plan.Config.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error setting app config", err.Error()+"\n\nThe app was installed and saved to state.")
		}
	}
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *appResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, found, err := r.client.GetApp(ctx, state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error reading app", err.Error())
		return
	} else if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(r.refresh(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *appResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.URL.Equal(state.URL) {
		if err := r.install(ctx, plan, true); err != nil {
			resp.Diagnostics.AddError("Error updating app", err.Error())
			return
		}
	}
	if known(plan.Config) && !sameConfig(plan.Config.ValueString(), state.Config.ValueString()) {
		if err := r.client.SetAppConfig(ctx, plan.Name.ValueString(), plan.Config.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error setting app config", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *appResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UninstallApp(ctx, state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error uninstalling app", err.Error())
	}
}

func (r *appResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

// refresh reads version, class paths and config; a config equal as JSON
// keeps the configured spelling.
func (r *appResource) refresh(ctx context.Context, m *appModel) diag.Diagnostics {
	var diags diag.Diagnostics
	app, found, err := r.client.GetApp(ctx, m.Name.ValueString())
	if err != nil || !found {
		diags.AddError("Error reading app", "app not found after install")
		return diags
	}
	cfg, err := r.client.GetAppConfig(ctx, app.Name)
	if err != nil {
		diags.AddError("Error reading app config", err.Error())
		return diags
	}

	paths := make([]string, 0, len(app.DnsApps))
	for _, d := range app.DnsApps {
		paths = append(paths, d.ClassPath)
	}
	cp, d := types.ListValueFrom(ctx, types.StringType, paths)
	diags.Append(d...)

	m.ID = types.StringValue(app.Name)
	m.Name = types.StringValue(app.Name)
	m.Version = types.StringValue(app.Version)
	m.ClassPaths = cp
	if !known(m.Config) || !sameConfig(m.Config.ValueString(), cfg) {
		m.Config = types.StringValue(cfg)
	}
	return diags
}
