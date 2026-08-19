package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AskUserTool cung cấp khả năng đặt câu hỏi làm rõ (clarifying questions)
// kèm danh sách lựa chọn (single/multi-select) khi brainstorm, lên kế hoạch hoặc thu thập yêu cầu.
type AskUserTool struct{}

// NewAskUserTool tạo instance mới của AskUserTool.
func NewAskUserTool() Tool {
	return &AskUserTool{}
}

func (t *AskUserTool) Name() string { return "ask_user" }

func (t *AskUserTool) Description() string {
	return "Đặt các câu hỏi làm rõ có chiều sâu (clarifying questions) kèm các phương án gợi ý (options) cho người dùng khi brainstorm, lập kế hoạch kiến trúc hoặc thu thập yêu cầu. Người dùng có thể chọn 1-click hoặc nhập tự do."
}

func (t *AskUserTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"questions":{
				"type":"array",
				"items":{
					"type":"object",
					"properties":{
						"id":{"type":"string","description":"Mã định danh cho câu hỏi"},
						"prompt":{"type":"string","description":"Nội dung câu hỏi cụ thể đặt ra cho người dùng"},
						"question":{"type":"string","description":"Tương đương prompt, câu hỏi làm rõ"},
						"header":{"type":"string","description":"Tiêu đề ngắn hoặc tag cho câu hỏi (tối đa 20 ký tự)"},
						"options":{
							"type":"array",
							"items":{
								"type":"object",
								"properties":{
									"label":{"type":"string","description":"Nhãn hiển thị cho lựa chọn"},
									"description":{"type":"string","description":"Mô tả ngắn giải thích ưu điểm/lý do cho lựa chọn"},
									"recommended":{"type":"boolean","description":"true nếu là phương án khuyến nghị"}
								},
								"required":["label"],
								"additionalProperties":false
							},
							"description":"2-4 phương án gợi ý sẵn cho người dùng"
						},
						"multi_select":{"type":"boolean","description":"true nếu cho phép người dùng chọn nhiều phương án cùng lúc"}
					},
					"additionalProperties":false
				},
				"description":"Danh sách các câu hỏi làm rõ có chiều sâu"
			}
		},
		"required":["questions"],
		"additionalProperties":false
	}`)
}

func (t *AskUserTool) Kind() Kind { return KindRead }

// AskUserArgs đại diện cho arguments gửi vào tool ask_user.
type AskUserArgs struct {
	Questions []struct {
		ID       string `json:"id,omitempty"`
		Prompt   string `json:"prompt"`
		Question string `json:"question,omitempty"`
		Header   string `json:"header,omitempty"`
		Options  []struct {
			Label       string `json:"label"`
			Description string `json:"description,omitempty"`
			Recommended bool   `json:"recommended,omitempty"`
		} `json:"options,omitempty"`
		MultiSelect bool `json:"multi_select,omitempty"`
	} `json:"questions"`
}

func (t *AskUserTool) Execute(_ context.Context, rawArgs json.RawMessage) (Result, error) {
	var args AskUserArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("ask_user: invalid args: %w", err)
	}

	if len(args.Questions) == 0 {
		return Result{}, fmt.Errorf("ask_user: questions array must not be empty")
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Đã gửi %d câu hỏi làm rõ cho người dùng:", len(args.Questions)))
	for i, q := range args.Questions {
		qText := q.Prompt
		if qText == "" {
			qText = q.Question
		}
		if qText == "" {
			qText = q.Header
		}
		summary.WriteString(fmt.Sprintf("\n%d. %s", i+1, qText))
		if len(q.Options) > 0 {
			opts := make([]string, 0, len(q.Options))
			for _, o := range q.Options {
				if o.Recommended {
					opts = append(opts, o.Label+" (Khuyến nghị)")
				} else {
					opts = append(opts, o.Label)
				}
			}
			summary.WriteString(fmt.Sprintf(" [Lựa chọn: %s]", strings.Join(opts, ", ")))
		}
	}

	return Result{
		Content: summary.String(),
		Meta:    rawArgs,
	}, nil
}
