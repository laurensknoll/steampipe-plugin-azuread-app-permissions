package azuread

import (
	"context"
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"

	"github.com/turbot/steampipe-plugin-sdk/v5/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/transform"
)

//// TABLE DEFINITION

func tableAzureAdServicePrincipalAppRoleAssignedTo(_ context.Context) *plugin.Table {
	return &plugin.Table{
		Name:        "azuread_service_principal_app_role_assigned_to",
		Description: "Represents an application role granted for a specific application. Includes the users and groups assigned app roles for this enterprise application.",
		Get: &plugin.GetConfig{
			Hydrate: getAdServicePrincipalAppRoleAssignedTo,
			IgnoreConfig: &plugin.IgnoreConfig{
				ShouldIgnoreErrorFunc: isIgnorableErrorPredicate([]string{"Request_ResourceNotFound", "Invalid object identifier"}),
			},
			KeyColumns: plugin.KeyColumnSlice{
				{Name: "service_principal_id", Require: plugin.Required},
				{Name: "id", Require: plugin.Required},
			},
		},
		List: &plugin.ListConfig{
			Hydrate: listAdServicePrincipalAppRoleAssignedTo,
			KeyColumns: plugin.KeyColumnSlice{
				// Key fields
				{Name: "service_principal_id", Require: plugin.Required},

				// Other fields for filtering OData
				{Name: "resource_id", Require: plugin.Optional},
				{Name: "principal_display_name", Require: plugin.Optional},
			},
		},

		Columns: []*plugin.Column{
			{Name: "id", Type: proto.ColumnType_STRING, Description: "A unique identifier for the appRoleAssignment key.", Transform: transform.FromMethod("GetId")},
			{Name: "app_role_id", Type: proto.ColumnType_STRING, Description: "The identifier (id) for the app role which is assigned to the principal. This app role must be exposed in the appRoles property on the resource application's service principal (resourceId). If the resource application has not declared any app roles, a default app role ID of 00000000-0000-0000-0000-000000000000 can be specified to signal that the principal is assigned to the resource app without any specific app roles.", Transform: transform.FromMethod("GetAppRoleId")},
			{Name: "resource_id", Type: proto.ColumnType_STRING, Description: "The unique identifier (id) for the resource service principal for which the assignment is made.", Transform: transform.FromMethod("GetResourceId")},
			{Name: "resource_display_name", Type: proto.ColumnType_STRING, Description: "The display name of the resource app's service principal to which the assignment is made.", Transform: transform.FromMethod("GetResourceDisplayName")},

			// Other fields
			{Name: "created_date_time", Type: proto.ColumnType_TIMESTAMP, Description: "The time when the app role assignment was created. The Timestamp type represents date and time information using ISO 8601 format and is always in UTC time. For example, midnight UTC on Jan 1, 2014 is 2014-01-01T00:00:00Z.", Transform: transform.FromMethod("GetCreatedDateTime")},
			{Name: "deleted_date_time", Type: proto.ColumnType_TIMESTAMP, Description: "The date and time when the app role assignment was deleted. Always null for an appRoleAssignment object that hasn't been deleted.", Transform: transform.FromMethod("GetDeletedDateTime")},

			{Name: "principal_id", Type: proto.ColumnType_STRING, Description: "The unique identifier (id) for the user, security group, or service principal being granted the app role.", Transform: transform.FromMethod("GetPrincipalId")},
			{Name: "principal_display_name", Type: proto.ColumnType_STRING, Description: "The display name of the user, group, or service principal that was granted the app role assignment.", Transform: transform.FromMethod("GetPrincipalDisplayName")},
			{Name: "principal_type", Type: proto.ColumnType_STRING, Description: "The type of the assigned principal. This can either be User, Group, or ServicePrincipal.", Transform: transform.FromMethod("GetPrincipalType")},

			// Standard columns
			{Name: "service_principal_id", Type: proto.ColumnType_STRING, Description: "The identifier (id) of the service principal.", Transform: transform.FromMethod("GetResourceId")},
		},
	}
}

//// LIST FUNCTION

func listAdServicePrincipalAppRoleAssignedTo(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (interface{}, error) {

	servicePrincipalId := d.EqualsQuals["service_principal_id"].GetStringValue()
	if servicePrincipalId == "" {
		return nil, nil
	}

	// Create client
	client, adapter, err := GetGraphClient(ctx, d)
	if err != nil {
		plugin.Logger(ctx).Error("azuread_service_principal_app_role_assigned_to.listAdServicePrincipalAppRoleAssignedTo", "connection_error", err)
		return nil, err
	}

	// List operations
	input := &serviceprincipals.ItemAppRoleAssignedToRequestBuilderGetQueryParameters{
		Top: Int32(999),
	}

	// Restrict the limit value to be passed in the query parameter which is not between 1 and 999, otherwise API will throw an error as follow
	// unexpected status 400 with OData error: Request_UnsupportedQuery: Invalid page size specified: '1000'. Must be between 1 and 999 inclusive.
	limit := d.QueryContext.Limit
	if limit != nil {
		if *limit > 0 && *limit < 999 {
			l := int32(*limit)
			input.Top = Int32(l)
		}
	}

	// Apply optional filters
	filters := buildAdServicePrincipalAppRoleAssignedToQueryFilter(d.Quals)
	if len(filters) > 0 {
		filterString := strings.Join(filters, " and ")
		input.Filter = &filterString
	}

	options := &serviceprincipals.ItemAppRoleAssignedToRequestBuilderGetRequestConfiguration{
		QueryParameters: input,
	}

	result, err := client.ServicePrincipals().ByServicePrincipalId(servicePrincipalId).AppRoleAssignedTo().Get(ctx, options)
	if err != nil {
		errObj := getErrorObject(err)
		plugin.Logger(ctx).Error("listAdServicePrincipalAppRoleAssignedTo", "list_service_principal_app_role_assigned_to_error", errObj)
		return nil, errObj
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.AppRoleAssignmentable](result, adapter, models.CreateAppRoleAssignmentCollectionResponseFromDiscriminatorValue)
	if err != nil {
		plugin.Logger(ctx).Error("listAdServicePrincipalAppRoleAssignedTo", "create_iterator_instance_error", err)
		return nil, err
	}

	err = pageIterator.Iterate(ctx, func(pageItem models.AppRoleAssignmentable) bool {
		d.StreamListItem(ctx, &ADAppRoleAssignmentInfo{pageItem})

		// Context can be cancelled due to manual cancellation or the limit has been hit
		return d.RowsRemaining(ctx) != 0
	})

	if err != nil {
		plugin.Logger(ctx).Error("listAdServicePrincipalAppRoleAssignedTo", "paging_error", err)
		return nil, err
	}

	return nil, nil
}

//// HYDRATE FUNCTIONS

func getAdServicePrincipalAppRoleAssignedTo(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {

	servicePrincipalId := d.EqualsQuals["service_principal_id"].GetStringValue()
	appRoleAssignmentId := d.EqualsQuals["id"].GetStringValue()
	if servicePrincipalId == "" || appRoleAssignmentId == "" {
		return nil, nil
	}

	// Create client
	client, _, err := GetGraphClient(ctx, d)
	if err != nil {
		plugin.Logger(ctx).Error("azuread_service_principal_app_role_assigned_to.getAdServicePrincipalAppRoleAssignedTo", "connection_error", err)
		return nil, err
	}

	appRoleAssignment, err := client.ServicePrincipals().ByServicePrincipalId(servicePrincipalId).AppRoleAssignedTo().ByAppRoleAssignmentId(appRoleAssignmentId).Get(ctx, nil)
	if err != nil {
		errObj := getErrorObject(err)
		plugin.Logger(ctx).Error("getAdServicePrincipalAppRoleAssignedTo", "get_service_principal_app_role_assigned_to_error", errObj)
		return nil, errObj
	}

	return &ADAppRoleAssignmentInfo{appRoleAssignment}, nil
}

//// TRANSFORM FUNCTIONS

func buildAdServicePrincipalAppRoleAssignedToQueryFilter(quals plugin.KeyColumnQualMap) []string {
	filters := []string{}

	filterTypes := map[string]string{
		"resource_id":            "guid",
		"principal_display_name": "string",
	}

	for k, v := range quals {
		if filterType, ok := filterTypes[k]; ok {
			for _, q := range v.Quals {
				switch filterType {
				case "guid":
					filters = append(filters, fmt.Sprintf("%s eq %s", strcase.ToCamel(k), q.Value.GetStringValue()))
				case "string":
					filters = append(filters, fmt.Sprintf("%s eq '%s'", strcase.ToCamel(k), q.Value.GetStringValue()))
				}
			}
		}
	}

	return filters
}
