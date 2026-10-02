// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// external_query_tool.go — v1.1.1 / DATA-09 + DATA-11 + DATA-13: the
// needs_external_query Tool, the only agent-visible entry into the
// External Authorized Channel. It mirrors kpi_tool.go's construction
// discipline: ONE Tool, ONE closure, catalog/schema snapshotted at
// CONSTRUCTION time, structured refusal as a TOOL RESULT never a Go
// error.
//
// Pass-through invariant (DATA-09, the reason this file exists under a
// structural lock): the Tool NEVER authors a query itself. Every
// preview is produced by nl2sql.Generate over the construction-time
// schema snapshot and forwarded verbatim into
// Manager.RequestApproval; execution happens only through a TTL token
// on a registered view. The structural lock in
// external_query_tool_test.go (comment-stripped zero mutation/verb
// tokens) plus internal/agent/sql_surface_test.go's WalkDir gate keep
// that invariant from drifting.
//
// Flow (see the case branches in invoke):
//
//   - no query_id  → nl2sql preview → Manager.RequestApproval →
//     status "pending" (DATA-11 preview + DATA-13 positive-pending).
//   - query_id, request pending → the pending shape again. Pending is
//     a positive state: the Tool never auto-resolves, never polls,
//     never blocks the call.
//   - query_id, request approved → Manager.ExecuteWithToken →
//     status "executed" with rows masked by the view's field masks
//     (DATA-04 + DATA-05). An expired token is a refusal — the agent
//     must re-request approval.
//   - query_id, request rejected → refusal carrying the officer's
//     decision. No token exists for a rejected request.
//
// UserID note: v1.1 has no authenticated-user concept, so the session
// id is stamped as the request's UserID. The TTL scope therefore binds
// (view, session) — a token minted for one session cannot execute in
// another. When real identity lands (v1.2), this stamp is the single
// line to change.
package agenttools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/externalauth"
	"github.com/skillforge/skillforge/internal/nl2sql"
)

// ExternalQueryToolName is the canonical Eino tool name, exported so
// internal/agent wiring and tests assert against the exact string
// rather than a magic literal (mirrors KPICallToolName).
const ExternalQueryToolName = "needs_external_query"

// ExternalQueryTool is an alias for the eino InvokableTool surface
// used by BuildExternalQueryTool (mirrors KPITool).
type ExternalQueryTool = interface {
	tool.BaseTool
	InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error)
}

// Refusal reason codes returned in the tool result. Structured, so the
// agent can narrate or self-correct (D16-B2 discipline applied to the
// external channel).
const (
	reasonNotConfigured   = "external_auth_not_configured"
	reasonInvalidArgs     = "invalid_arguments"
	reasonRequestNotFound = "request_not_found"
	reasonRequestRejected = "request_rejected"
	reasonTokenExpired    = "token_expired"
	reasonPreviewRejected = "sql_preview_rejected"
	reasonViewNotAllowed  = "view_not_allowed"
	reasonRequestFailed   = "external_auth_request_failed"
)

// ExternalQueryToolConfig carries the channel seams the Tool closes
// over. Everything is snapshotted at construction time: the NL2SQL
// schema and the per-view mask list are derived from the registry
// snapshot ONCE, so a later registry change would require building a
// NEW Tool (construction-time snapshot discipline — Don't-Hand-Roll).
type ExternalQueryToolConfig struct {
	// Manager is the channel orchestrator. nil is a valid construction
	// input ONLY in hermetic tests: the Tool answers every call with a
	// structured external_auth_not_configured refusal (defense in depth
	// — BuildRunner already fail-fasts on nil before attaching this
	// Tool).
	Manager *externalauth.Manager

	// ChatModel is the model nl2sql uses to draft the preview. Same
	// model the agent already uses (see the nl2sql package doc for the
	// v1.1 rationale).
	ChatModel model.ToolCallingChatModel

	// Registry is the DATA-10 view allowlist snapshot. When nil the
	// schema is empty and every request is rejected view-not-allowed
	// downstream — fail-closed, never fail-open.
	Registry *externalauth.ViewRegistry

	// SessionID + SkillID are stamped into every ExternalAuthRequest so
	// the dual audit (DATA-06/DATA-14) carries the ambient identity.
	SessionID string
	SkillID   string
}

// externalQueryArgs is the JSON arguments shape the LLM emits.
type externalQueryArgs struct {
	// Query is the user's natural-language request (zh-CN or en). It is
	// the nl2sql input AND the business_reason fallback — the data
	// officer reads it before deciding.
	Query string `json:"query"`
	// BusinessReason is the stated reason for the access. Optional;
	// defaults to the query text.
	BusinessReason string `json:"business_reason"`
	// QueryID is the xreq-* id returned by a previous pending call.
	// Present on the second invocation, after the data officer decided.
	QueryID string `json:"query_id"`
}

// externalQueryResponse is the single JSON tool-result shape across all
// branches (refused | pending | executed), discriminated by Status.
type externalQueryResponse struct {
	Status           string              `json:"status"`
	ReasonCode       string              `json:"reason_code,omitempty"`
	HumanExplanation string              `json:"human_explanation,omitempty"`
	QueryID          string              `json:"query_id,omitempty"`
	SQLPreview       string              `json:"sql_preview,omitempty"`
	View             string              `json:"view,omitempty"`
	EstimatedRows    int                 `json:"estimated_rows,omitempty"`
	Rows             []map[string]string `json:"rows,omitempty"`
	Count            int                 `json:"count,omitempty"`
	MaskApplied      []string            `json:"mask_applied,omitempty"`
	Message          string              `json:"message,omitempty"`
}

// BuildExternalQueryTool constructs the needs_external_query
// InvokableTool. See the file doc for the flow contract and the
// pass-through invariant.
func BuildExternalQueryTool(cfg ExternalQueryToolConfig) tool.InvokableTool {
	// Construction-time snapshots (Don't-Hand-Roll): the NL2SQL schema
	// and the per-view mask-field lists are derived ONCE from the
	// registry snapshot and frozen into the closure.
	schemaSnapshot := nl2sql.Schema{}
	maskSnapshot := map[string][]string{}
	if cfg.Registry != nil {
		for _, name := range cfg.Registry.Names() {
			v, ok := cfg.Registry.Get(name)
			if !ok {
				continue
			}
			cols := make([]nl2sql.ColumnSpec, 0, len(v.Columns))
			for _, c := range v.Columns {
				cols = append(cols, nl2sql.ColumnSpec{Name: c.Name, Type: c.Type})
			}
			schemaSnapshot.Views = append(schemaSnapshot.Views, nl2sql.ViewSchema{ViewName: v.ViewName, Columns: cols})
			masks := make([]string, 0, len(v.FieldMasks))
			for _, m := range v.FieldMasks {
				masks = append(masks, m.FieldName)
			}
			maskSnapshot[v.ViewName] = masks
		}
	}

	desc := &schema.ToolInfo{
		Name: ExternalQueryToolName,
		Desc: "发起外部数据查询（个体级数据，需数据人员审批）。第一次调用只带 query：生成只读预览并进入人工审批，返回 pending 状态与 query_id；数据人员批准后，再次调用并携带 query_id 取回按视图掩码规则脱敏的行。任何步骤都不会返回未脱敏数据。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Required: true,
				Desc:     "用户的自然语言数据请求（同时作为审批页面展示的业务理由兜底）",
			},
			"business_reason": {
				Type:     schema.String,
				Required: false,
				Desc:     "访问该数据的业务理由；缺省时使用 query 原文",
			},
			"query_id": {
				Type:     schema.String,
				Required: false,
				Desc:     "上一次 pending 返回的审批单号（xreq- 开头）；携带后按审批状态执行或再次报告 pending",
			},
		}),
	}

	invoke := func(ctx context.Context, args externalQueryArgs) (externalQueryResponse, error) {
		// ① Fail-closed preconditions — structured refusals, never Go
		// errors (D16-B2 applied to the channel).
		if cfg.Manager == nil {
			return refused(reasonNotConfigured, "外部授权通道未装配（服务端未启用）——本 Skill 声明了 external:authorized，但服务没有启用外部授权通道。"), nil
		}
		query := strings.TrimSpace(args.Query)
		if query == "" {
			return refused(reasonInvalidArgs, "query 不能为空：请描述要查询的数据与业务理由。"), nil
		}

		// ② query_id branch — the post-approval continuation.
		if id := strings.TrimSpace(args.QueryID); id != "" {
			req, ok, err := cfg.Manager.Store().GetExternalAuthRequest(ctx, id)
			if err != nil {
				return refused(reasonRequestFailed, "查询审批单失败："+err.Error()), nil
			}
			if !ok {
				return refused(reasonRequestNotFound, fmt.Sprintf("审批单 %s 不存在：请核对 query_id，或重新发起一次新的审批请求。", id)), nil
			}
			switch req.State {
			case externalauth.StatePending:
				// DATA-13: pending is a positive state. Never
				// auto-resolve, never poll, never block the call.
				return externalQueryResponse{
					Status:        "pending",
					QueryID:       req.RequestID,
					SQLPreview:    req.Preview.SQLPreview,
					View:          req.Preview.ViewName,
					EstimatedRows: req.Preview.EstimatedRows,
					MaskApplied:   maskSnapshot[req.Preview.ViewName],
					Message:       "该审批单仍在等待数据人员处理。请把审批卡片呈现给用户并等待；不要轮询，不要重复提交新的审批单。",
				}, nil
			case externalauth.StateApproved:
				rows, execErr := cfg.Manager.ExecuteWithToken(ctx, req.TTLTokenID)
				if execErr != nil {
					// An expired (or otherwise unusable) token is a
					// refusal: the agent must re-request approval, never
					// retry the same token.
					detail := "审批凭证不可用"
					if errors.Is(execErr, externalauth.ErrTokenExpired) {
						detail = "审批凭证已过期（ttl 到期）"
					}
					return refused(reasonTokenExpired, fmt.Sprintf("%s：%v。审批单 %s 已失效，请重新发起一次新的审批请求。", detail, execErr, req.RequestID)), nil
				}
				return externalQueryResponse{
					Status:      "executed",
					QueryID:     req.RequestID,
					Rows:        rows,
					Count:       len(rows),
					MaskApplied: maskSnapshot[req.Preview.ViewName],
					Message:     "返回的行已按视图掩码规则脱敏；请如实说明字段被脱敏，不得尝试还原。",
				}, nil
			default:
				// rejected — the officer's decision is the answer; no
				// token exists for a rejected request.
				return refused(reasonRequestRejected, fmt.Sprintf("审批单 %s 已被数据人员拒绝。不得再次提交等价请求，请向用户说明拒绝结果。", req.RequestID)), nil
			}
		}

		// ③ No query_id — pass-through preview generation (DATA-09: the
		// Tool never authors a query itself) then the approval request.
		preview, err := nl2sql.Generate(ctx, cfg.ChatModel, schemaSnapshot, query)
		if err != nil {
			return refused(reasonPreviewRejected, fmt.Sprintf("预览生成被拒绝：%v。请调整问法后重试；不要请求只读白名单之外的数据。", err)), nil
		}
		reason := strings.TrimSpace(args.BusinessReason)
		if reason == "" {
			reason = query
		}
		created, err := cfg.Manager.RequestApproval(ctx, externalauth.ExternalAuthRequest{
			// v1.1: no authenticated-user concept — the session id is the
			// UserID, so the TTL scope binds (view, session).
			UserID:    cfg.SessionID,
			SkillID:   cfg.SkillID,
			SessionID: cfg.SessionID,
			Preview:   preview,
		})
		if err != nil {
			if errors.Is(err, externalauth.ErrViewNotAllowed) {
				return refused(reasonViewNotAllowed, fmt.Sprintf("视图不在授权清单内：%v。只能查询 external_views/ 受控文件中登记的视图。", err)), nil
			}
			return refused(reasonRequestFailed, fmt.Sprintf("创建审批单失败：%v", err)), nil
		}
		return externalQueryResponse{
			Status:        "pending",
			QueryID:       created.RequestID,
			SQLPreview:    created.Preview.SQLPreview,
			View:          created.Preview.ViewName,
			EstimatedRows: created.Preview.EstimatedRows,
			MaskApplied:   maskSnapshot[created.Preview.ViewName],
			Message:       "已创建审批单并进入等待审批状态。请向用户呈现待审批卡片；数据人员批准后，携带该 query_id 再次调用本工具取回脱敏行。",
		}, nil
	}

	return utils.NewTool(desc, invoke)
}

// refused is the structured-refusal constructor (D16-B2: a refusal is
// a tool result, never a Go error).
func refused(reasonCode, explanation string) externalQueryResponse {
	return externalQueryResponse{
		Status:           "refused",
		ReasonCode:       reasonCode,
		HumanExplanation: explanation,
	}
}
