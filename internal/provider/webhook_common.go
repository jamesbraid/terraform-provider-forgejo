package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var webhookWriteOnlyConfigKeys = []string{"secret"}

type webhookModelValues struct {
	WebhookID           types.Int64
	Active              types.Bool
	AuthorizationHeader types.String
	BranchFilter        types.String
	Config              types.Map
	CreatedAt           types.String
	Events              types.Set
	Type                types.String
	UpdatedAt           types.String
}

func webhookResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"webhook_id": schema.Int64Attribute{
			Description: "Numeric identifier of the webhook.",
			Computed:    true,
			PlanModifiers: []planmodifier.Int64{
				int64planmodifier.UseStateForUnknown(),
			},
		},
		"active": schema.BoolAttribute{
			Description: "Boolean indicating if the webhook is active.",
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(false),
		},
		"authorization_header": schema.StringAttribute{
			Description: "Authorization header to send to the target.",
			Optional:    true,
			Sensitive:   true,
		},
		"branch_filter": schema.StringAttribute{
			Description: "Allowed branches for push, branch creation, and branch deletion events, specified as a glob pattern.",
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(""),
		},
		"config": schema.MapAttribute{
			Description: "Webhook configuration. The secret key is write-only and cannot be checked for out-of-band changes.",
			ElementType: types.StringType,
			Required:    true,
		},
		"created_at": schema.StringAttribute{
			Description: "Time at which the webhook was created.",
			Computed:    true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"events": schema.SetAttribute{
			Description: "Events which trigger the webhook.",
			ElementType: types.StringType,
			Optional:    true,
			Computed:    true,
			Default: setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("push"),
			})),
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(stringvalidator.OneOf(
					"action_run_failure", "action_run_recover", "action_run_success",
					"create", "delete", "fork", "issue_assign", "issue_comment",
					"issue_label", "issue_milestone", "issues", "package",
					"pull_request", "pull_request_assign", "pull_request_comment",
					"pull_request_label", "pull_request_milestone",
					"pull_request_review_approved", "pull_request_review_comment",
					"pull_request_review_rejected", "pull_request_review_request",
					"pull_request_sync", "push", "release", "repository", "wiki",
				)),
			},
		},
		"type": schema.StringAttribute{
			Description: "Type of webhook. Changing this forces a new resource to be created.",
			Required:    true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
			Validators: []validator.String{
				stringvalidator.OneOf("dingtalk", "discord", "feishu", "forgejo", "gitea", "gogs", "msteams", "slack", "telegram"),
			},
		},
		"updated_at": schema.StringAttribute{
			Description: "Time at which the webhook was updated.",
			Computed:    true,
		},
	}
}

func normalizedWebhookEvents(ctx context.Context, prior types.Set, actual []string) (types.Set, diag.Diagnostics) {
	actual = canonicalWebhookEvents(actual)
	if !prior.IsNull() && !prior.IsUnknown() {
		var declared []string
		diags := prior.ElementsAs(ctx, &declared, false)
		if diags.HasError() {
			return types.SetNull(types.StringType), diags
		}

		actualSet := make(map[string]struct{}, len(actual))
		for _, event := range actual {
			actualSet[event] = struct{}{}
		}
		allPresent := true
		for _, event := range declared {
			if _, ok := actualSet[event]; !ok {
				allPresent = false
				break
			}
		}
		if allPresent {
			return prior, diags
		}
	}

	return types.SetValueFrom(ctx, types.StringType, actual)
}

func canonicalWebhookEvents(events []string) []string {
	hasPullRequest := false
	for _, event := range events {
		if event == "pull_request" {
			hasPullRequest = true
			break
		}
	}
	if !hasPullRequest {
		return events
	}

	result := make([]string, 0, len(events))
	for _, event := range events {
		if strings.HasPrefix(event, "pull_request_") {
			continue
		}
		result = append(result, event)
	}
	return result
}

func webhookValuesFromHook(ctx context.Context, prior webhookModelValues, hook *forgejo.Hook) (webhookModelValues, diag.Diagnostics) {
	if hook == nil {
		return prior, nil
	}

	config := make(map[string]string, len(hook.Config))
	for key, value := range hook.Config {
		config[key] = value
	}
	var priorConfig map[string]string
	var diags diag.Diagnostics
	if !prior.Config.IsNull() && !prior.Config.IsUnknown() {
		diags.Append(prior.Config.ElementsAs(ctx, &priorConfig, false)...)
	}
	for _, key := range webhookWriteOnlyConfigKeys {
		delete(config, key)
		if value, ok := priorConfig[key]; ok {
			config[key] = value
		}
	}

	result := prior
	result.WebhookID = types.Int64Value(hook.ID)
	result.Active = types.BoolValue(hook.Active)
	var convertedDiags diag.Diagnostics
	result.Config, convertedDiags = types.MapValueFrom(ctx, types.StringType, config)
	diags.Append(convertedDiags...)
	result.CreatedAt = types.StringValue(hook.Created.Format(time.RFC3339))
	result.Events, convertedDiags = normalizedWebhookEvents(ctx, prior.Events, hook.Events)
	diags.Append(convertedDiags...)
	result.Type = types.StringValue(hook.Type)
	result.UpdatedAt = types.StringValue(hook.Updated.Format(time.RFC3339))

	return result, diags
}

func (v webhookModelValues) createOption(ctx context.Context) (forgejo.CreateHookOption, diag.Diagnostics) {
	var config map[string]string
	var events []string
	var diags diag.Diagnostics
	diags.Append(v.Config.ElementsAs(ctx, &config, false)...)
	diags.Append(v.Events.ElementsAs(ctx, &events, false)...)
	return forgejo.CreateHookOption{
		Type:                forgejo.HookType(v.Type.ValueString()),
		Config:              config,
		Events:              events,
		BranchFilter:        v.BranchFilter.ValueString(),
		Active:              v.Active.ValueBool(),
		AuthorizationHeader: v.AuthorizationHeader.ValueString(),
	}, diags
}

func (v webhookModelValues) editOption(ctx context.Context) (forgejo.EditHookOption, diag.Diagnostics) {
	created, diags := v.createOption(ctx)
	return forgejo.EditHookOption{
		Config:              created.Config,
		Events:              created.Events,
		BranchFilter:        created.BranchFilter,
		Active:              v.Active.ValueBoolPointer(),
		AuthorizationHeader: created.AuthorizationHeader,
	}, diags
}

func redactWebhookConfig(config map[string]string) map[string]string {
	redacted := make(map[string]string, len(config))
	for key, value := range config {
		redacted[key] = value
	}
	for _, key := range webhookWriteOnlyConfigKeys {
		if value, ok := redacted[key]; ok {
			redacted[key] = strings.Repeat("*", len(value))
		}
	}
	return redacted
}

func forgejoAPIError(response *forgejo.Response, err error) string {
	if response == nil {
		return fmt.Sprintf("unknown error with nil response: %s", err)
	}
	return fmt.Sprintf("Forgejo returned status %d: %s", response.StatusCode, err)
}
