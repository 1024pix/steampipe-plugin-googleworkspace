package googleworkspace

import (
    "context"
    "time"

    "github.com/turbot/steampipe-plugin-sdk/v5/grpc/proto"
    "github.com/turbot/steampipe-plugin-sdk/v5/plugin"
    "github.com/turbot/steampipe-plugin-sdk/v5/plugin/transform"
    "google.golang.org/api/admin/reports/v1"
)

//// TABLE DEFINITION

func tableGoogleworkspaceAdminReportsDriveActivity(ctx context.Context) *plugin.Table {
    return &plugin.Table{
        Name:        "googleworkspace_admin_reports_drive_activity",
        Description: "Google Drive activity events: file views, edits, downloads, etc.",

        List: &plugin.ListConfig{
            Hydrate: listGoogleworkspaceAdminReportsDriveActivities,
            KeyColumns: plugin.KeyColumnSlice{
                {Name: "time", Require: plugin.Optional, Operators: []string{">", ">=", "<", "<=", "="}},
                {Name: "actor_email", Require: plugin.Optional},
                {Name: "ip_address", Require: plugin.Optional},
                {Name: "event_name", Require: plugin.Optional},
            },
            Tags: map[string]string{"service": "admin", "product": "reports", "action": "activities.list"},
        },
        Columns: []*plugin.Column{
            {
                Name:        "time",
                Description: "Timestamp of the activity (Id.Time) in RFC3339 format",
                Type:        proto.ColumnType_TIMESTAMP,
                Transform:   transform.FromField("Id.Time"),
            },
            {
                Name:        "actor_email",
                Description: "Email address of the actor (Actor.Email)",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("Actor.Email"),
            },
            {
                Name:        "file_name",
                Description: "Name of the file (optional)",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("Events").Transform(extractFileName),
            },
            {
                Name:        "event_name",
                Description: "List of event names for this activity",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("Events").Transform(extractEventNames),
            },
            {
                Name:        "unique_qualifier",
                Description: "Unique qualifier ID for this activity (Id.UniqueQualifier)",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("Id.UniqueQualifier"),
            },
            {
                Name:        "application_name",
                Description: "Name of the report application (Id.ApplicationName)",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("Id.ApplicationName"),
            },
            {
                Name:        "actor_profile_id",
                Description: "Profile ID of the actor (Actor.ProfileId)",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("Actor.ProfileId"),
            },
            {
                Name:        "actor_caller_type",
                Description: "Caller type of the actor (Actor.CallerType)",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("Actor.CallerType"),
            },
            {
                Name:        "ip_address",
                Description: "IP address associated with the activity (IpAddress)",
                Type:        proto.ColumnType_STRING,
                Transform:   transform.FromField("IpAddress"),
            },
            {
                Name:        "events",
                Description: "Full JSON array of detailed events (Events)",
                Type:        proto.ColumnType_JSON,
                Transform:   transform.FromField("Events"),
            },
        },
    }
}

//// LIST FUNCTION

func listGoogleworkspaceAdminReportsDriveActivities(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (interface{}, error) {
    service, err := ReportsService(ctx, d)
    if err != nil {
        plugin.Logger(ctx).Error("googleworkspace_admin_reports_drive_activity.list", "service_error", err)
        return nil, err
    }

    call := service.Activities.List("all", "drive")

    // If the user supplied a time qualifier, translate it to StartTime/EndTime parameters
    if quals := d.Quals["time"]; quals != nil {
        var startTime, endTime time.Time
        for _, q := range quals.Quals {
            if ts := q.Value.GetTimestampValue(); ts != nil {
                t := ts.AsTime()
                switch q.Operator {
                case "=":
                    startTime, endTime = t, t
                case ">":
                    startTime = t.Add(time.Nanosecond)
                case ">=":
                    startTime = t
                case "<":
                    endTime = t
                case "<=":
                    endTime = t
                }
            }
        }
        if !startTime.IsZero() {
            call.StartTime(startTime.Format(time.RFC3339))
        }
        if !endTime.IsZero() {
            call.EndTime(endTime.Format(time.RFC3339))
        }
    }

    // Pagination setup
    pageToken := ""
    const apiMaxPageSize = 1000

    // Determine initial page size based on SQL LIMIT
    var initialPageSize int64 = apiMaxPageSize
    if limit := d.QueryContext.Limit; limit != nil && *limit < initialPageSize {
        initialPageSize = *limit
    }
    call.MaxResults(initialPageSize)

    for {
        if pageToken != "" {
            call.PageToken(pageToken)
        }
        resp, err := call.Do()
        if err != nil {
            plugin.Logger(ctx).Error("googleworkspace_admin_reports_drive_activity.list", "api_error", err)
            return nil, err
        }
        for _, activity := range resp.Items {
            d.StreamListItem(ctx, activity)
            if d.RowsRemaining(ctx) == 0 {
                return nil, nil
            }
        }
        if resp.NextPageToken == "" {
            break
        }
        pageToken = resp.NextPageToken

        if limit := d.QueryContext.Limit; limit != nil {
            remaining := d.RowsRemaining(ctx)
            if remaining > 0 && remaining < apiMaxPageSize {
                call.MaxResults(int64(remaining))
            } else {
                call.MaxResults(apiMaxPageSize)
            }
        } else {
            call.MaxResults(apiMaxPageSize)
        }
    }

    return nil, nil
}

/// TRANSFORM FUNCTIONS

func extractFileName(_ context.Context, d *transform.TransformData) (interface{}, error) {
    activity, ok := d.HydrateItem.(*admin.Activity)
    if !ok {
        return nil, nil
    }
    for _, event := range activity.Events {
        if event.Parameters != nil {
            for _, p := range event.Parameters {
                if p.Name == "doc_title" {
                    return p.Value, nil
                }
            }
        }
    }
    return nil, nil
}
