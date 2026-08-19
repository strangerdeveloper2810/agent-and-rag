package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ClarifyOption đại diện cho một phương án lựa chọn trong câu hỏi làm rõ.
type ClarifyOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

// ClarifyQuestion đại diện cho một câu hỏi làm rõ (tương tự agent-toolkit ask_user).
type ClarifyQuestion struct {
	ID          string          `json:"id,omitempty"`
	Prompt      string          `json:"prompt"`
	Question    string          `json:"question,omitempty"`
	Header      string          `json:"header,omitempty"`
	Options     []ClarifyOption `json:"options,omitempty"`
	MultiSelect bool            `json:"multiSelect,omitempty"`
}

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
			"prompt":{"type":"string","description":"Nội dung câu hỏi chi tiết đặt ra cho người dùng"},
			"question":{"type":"string","description":"Tương đương prompt, nội dung câu hỏi chi tiết"},
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
			"multi_select":{"type":"boolean","description":"true nếu cho phép người dùng chọn nhiều phương án cùng lúc"},
			"questions":{
				"type":"array",
				"items":{
					"type":"object",
					"properties":{
						"id":{"type":"string","description":"Mã định danh cho câu hỏi"},
						"prompt":{"type":"string","description":"Nội dung câu hỏi chi tiết đặt ra cho người dùng"},
						"question":{"type":"string","description":"Tương đương prompt, nội dung câu hỏi chi tiết"},
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
				"description":"Danh sách các câu hỏi làm rõ nếu gửi nhiều câu hỏi cùng lúc"
			}
		},
		"additionalProperties":false
	}`)
}

func (t *AskUserTool) Kind() Kind { return KindRead }

type rawQuestionItem struct {
	ID               string          `json:"id,omitempty"`
	Prompt           string          `json:"prompt,omitempty"`
	Question         string          `json:"question,omitempty"`
	Header           string          `json:"header,omitempty"`
	Options          json.RawMessage `json:"options,omitempty"`
	Choices          json.RawMessage `json:"choices,omitempty"`
	MultiSelect      bool            `json:"multi_select,omitempty"`
	MultiSelectCamel bool            `json:"multiSelect,omitempty"`
}

func parseOptions(raw json.RawMessage) []ClarifyOption {
	if len(raw) == 0 {
		return nil
	}
	// Try parsing as []ClarifyOption
	var opts []ClarifyOption
	if err := json.Unmarshal(raw, &opts); err == nil && len(opts) > 0 && opts[0].Label != "" {
		return opts
	}
	// Try parsing as []string
	var strOpts []string
	if err := json.Unmarshal(raw, &strOpts); err == nil {
		res := make([]ClarifyOption, 0, len(strOpts))
		for _, s := range strOpts {
			s = strings.TrimSpace(s)
			if s != "" {
				res = append(res, ClarifyOption{Label: s})
			}
		}
		return res
	}
	return opts
}

// ParseClarifyQuestions giải mã linh hoạt mọi định dạng câu hỏi do LLM sinh ra:
// 1. Dạng bọc array: {"questions": [...]}
// 2. Dạng array trực tiếp: [...]
// 3. Dạng single question: {"prompt": "...", "header": "...", "options": [...]}
func ParseClarifyQuestions(rawArgs json.RawMessage) []ClarifyQuestion {
	if len(rawArgs) == 0 {
		return nil
	}

	var items []rawQuestionItem

	// Case 1: {"questions": [...]}
	var wrapper struct {
		Questions []rawQuestionItem `json:"questions"`
	}
	if err := json.Unmarshal(rawArgs, &wrapper); err == nil && len(wrapper.Questions) > 0 {
		items = wrapper.Questions
	} else {
		// Case 2: Array at top-level [...]
		var arr []rawQuestionItem
		if err := json.Unmarshal(rawArgs, &arr); err == nil && len(arr) > 0 {
			items = arr
		} else {
			// Case 3: Single question object at top-level {...}
			var single rawQuestionItem
			if err := json.Unmarshal(rawArgs, &single); err == nil {
				if single.Prompt != "" || single.Question != "" || single.Header != "" || len(single.Options) > 0 {
					items = []rawQuestionItem{single}
				}
			}
		}
	}

	res := make([]ClarifyQuestion, 0, len(items))
	for _, it := range items {
		prompt := strings.TrimSpace(it.Prompt)
		if prompt == "" {
			prompt = strings.TrimSpace(it.Question)
		}
		if prompt == "" {
			prompt = strings.TrimSpace(it.Header)
		}

		opts := parseOptions(it.Options)
		if len(opts) == 0 {
			opts = parseOptions(it.Choices)
		}

		res = append(res, ClarifyQuestion{
			ID:          it.ID,
			Prompt:      prompt,
			Question:    prompt,
			Header:      it.Header,
			Options:     opts,
			MultiSelect: it.MultiSelect || it.MultiSelectCamel,
		})
	}

	return res
}

func (t *AskUserTool) Execute(_ context.Context, rawArgs json.RawMessage) (Result, error) {
	questions := ParseClarifyQuestions(rawArgs)
	if len(questions) == 0 {
		return Result{}, fmt.Errorf("ask_user: could not parse question from args: %s", string(rawArgs))
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Đã gửi %d câu hỏi làm rõ cho người dùng:", len(questions)))
	for i, q := range questions {
		summary.WriteString(fmt.Sprintf("\n%d. %s", i+1, q.Prompt))
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
