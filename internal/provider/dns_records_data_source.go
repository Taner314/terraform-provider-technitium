package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &dnsRecordsDataSource{}

func NewDnsRecordsDataSource() datasource.DataSource { return &dnsRecordsDataSource{} }

type dnsRecordsDataSource struct {
	client *technitium.Client
}

type dnsRecordsModel struct {
	Zone    types.String `tfsdk:"zone"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Records types.List   `tfsdk:"records"`
}

var dnsRecordType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":     types.StringType,
	"fqdn":     types.StringType,
	"type":     types.StringType,
	"ttl":      types.Int64Type,
	"disabled": types.BoolType,
	"comments": types.StringType,
	"data":     types.MapType{ElemType: types.StringType},
}}

func (d *dnsRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *dnsRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = ConfigureDataSourceClient(req, resp)
}

func (d *dnsRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists records in a zone, optionally filtered by name and type.",
		Attributes: map[string]schema.Attribute{
			"zone": schema.StringAttribute{Required: true, Description: "Zone name."},
			"name": schema.StringAttribute{Optional: true, Description: "Only records with this zone-relative name (`@` for the apex)."},
			"type": schema.StringAttribute{Optional: true, Description: "Only records of this type, e.g. `A`."},
			"records": schema.ListAttribute{Computed: true, ElementType: dnsRecordType,
				Description: "Records: `name` (zone-relative), `fqdn`, `type`, `ttl`, `disabled`, `comments` and `data` " +
					"(the record's fields as strings, e.g. `ipAddress`, `exchange`; structured values JSON-encoded)."},
		},
	}
}

func (d *dnsRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m dnsRecordsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := strings.TrimSuffix(m.Zone.ValueString(), ".")
	domain, listZone := zone, true
	if known(m.Name) && m.Name.ValueString() != "" {
		domain, listZone = fqdn(m.Name.ValueString(), zone), false
	}

	recs, err := d.client.GetRecords(ctx, zone, domain, listZone)
	if err != nil {
		resp.Diagnostics.AddError("Error listing records", err.Error())
		return
	}

	elems := make([]attr.Value, 0, len(recs))
	for _, r := range recs {
		if known(m.Type) && m.Type.ValueString() != "" && !strings.EqualFold(r.Type, m.Type.ValueString()) {
			continue
		}
		data := map[string]attr.Value{}
		keys := make([]string, 0, len(r.RData))
		for k := range r.RData {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			data[k] = types.StringValue(rdataString(r.RData[k]))
		}
		obj, diags := types.ObjectValue(dnsRecordType.AttrTypes, map[string]attr.Value{
			"name":     types.StringValue(relativeName(r.Name, zone)),
			"fqdn":     types.StringValue(r.Name),
			"type":     types.StringValue(r.Type),
			"ttl":      types.Int64Value(r.TTL),
			"disabled": types.BoolValue(r.Disabled),
			"comments": types.StringValue(r.Comments),
			"data":     types.MapValueMust(types.StringType, data),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	list, diags := types.ListValue(dnsRecordType, elems)
	resp.Diagnostics.Append(diags...)
	m.Records = list
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func rdataString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	case float64, bool:
		return fmt.Sprint(x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}
