// Command prompt-size-probe đo kích thước THẬT của từng thành phần trong request
// gửi lên LLM: system prompt, skill được kích hoạt, tool schema, memory.
//
// Lý do tồn tại: log production báo tokens_in=41400 cho một câu chat đơn giản,
// nhưng đọc code thì không thấy chỗ nào đủ lớn để giải thích con số đó. Đoán
// tiếp là vô nghĩa — đo rồi mới sửa.
//
//	go run ./internal-prompt-size-probe
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/memory"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// approxTokens là ước lượng thô: ~3,5 byte/token cho văn bản tiếng Việt trộn
// tiếng Anh (tiếng Việt có dấu tốn byte hơn ASCII). Đủ chính xác để so sánh
// tương đối giữa các thành phần.
func approxTokens(n int) int { return n * 10 / 35 }

func line(label string, bytes int) {
	fmt.Printf("  %-38s %8d byte  ≈ %7d token\n", label, bytes, approxTokens(bytes))
}

func main() {
	skillsDir := os.Getenv("JARVIS_SKILLS_DIR")
	if skillsDir == "" {
		skillsDir = "./skills"
	}

	loader, err := skills.NewLoader(skillsDir)
	if err != nil {
		fmt.Println("không load được skills:", err)
		os.Exit(1)
	}
	summaries := loader.ListSkills()

	fmt.Printf("\n═══ Thành phần system prompt ═══\n")
	base := agent.BuildSystemPrompt(nil, summaries, "vi")
	line(fmt.Sprintf("base prompt + %d skill summary", len(summaries)), len(base))

	bare := agent.BuildSystemPrompt(nil, nil, "vi")
	line("chỉ base prompt (không skill list)", len(bare))
	line("phần skill summary", len(base)-len(bare))

	// Skill được kích hoạt: toàn bộ SKILL.md được nối vào prompt.
	if sk := loader.MatchSkill("giải thích giúp tôi RAG là gì cho người mới học"); sk != nil {
		// PromptBody() = đúng thứ production chèn vào prompt (đã bỏ frontmatter
		// và gọt theo skills.MaxPromptBytes), không phải nội dung file thô.
		line("skill kích hoạt: "+sk.Name, len(sk.PromptBody()))
		line("  (file gốc, chưa gọt)", len(sk.Content))
	}

	// Tool schema — đây là phần thường bị bỏ qua khi tính token.
	store := memory.NewStore()
	reg := tools.NewRegistry()
	reg.Register(tools.NewEchoTool())
	reg.Register(tools.NewFileSearchTool(nil))
	reg.Register(tools.NewFileReadTool(nil))
	reg.Register(tools.NewFileWriteTool(nil))
	reg.Register(tools.NewShellToolWithTimeout(nil, 30*time.Second))
	reg.Register(tools.NewWebSearchTool(nil))
	reg.Register(tools.NewWebFetchTool(nil))
	reg.Register(tools.NewTranslateTool(nil))
	reg.Register(tools.NewCalculatorTool())
	reg.Register(tools.NewDateTimeTool())
	reg.Register(tools.NewSaveMemoryTool(store))
	reg.Register(tools.NewRecallMemoryTool(store))
	reg.Register(tools.NewListMemoriesTool(store))

	// Skill nào đang bị trần cắt: đây là chỗ mất nội dung, cần viết lại cho gọn.
	fmt.Printf("\n═══ Skill vượt trần %d byte (bị gọt) ═══\n", skills.MaxPromptBytes)
	overCount, overBytes := 0, 0
	type overSkill struct {
		name string
		size int
	}
	var over []overSkill
	for _, sum := range summaries {
		sk := loader.LoadSkill(sum.Name)
		if sk == nil {
			continue
		}
		if len(sk.Content) > skills.MaxPromptBytes {
			over = append(over, overSkill{sum.Name, len(sk.Content)})
			overCount++
			overBytes += len(sk.Content) - skills.MaxPromptBytes
		}
	}
	sort.Slice(over, func(i, j int) bool { return over[i].size > over[j].size })
	for _, o := range over {
		fmt.Printf("  %-28s %6d byte  (mất %d byte)\n", o.name, o.size, o.size-skills.MaxPromptBytes)
	}
	if overCount == 0 {
		fmt.Printf("  không có skill nào bị gọt ✅\n")
	} else {
		fmt.Printf("  → %d/%d skill bị gọt, tổng %d byte nội dung không tới được model\n",
			overCount, len(summaries), overBytes)
	}

	fmt.Printf("\n═══ Tool schema ═══\n")
	all := reg.ToolDefs()
	totalAll := 0
	sizes := map[string]int{}
	for _, d := range all {
		b, _ := json.Marshal(d)
		sizes[d.Name] = len(b)
		totalAll += len(b)
	}
	line(fmt.Sprintf("TẤT CẢ %d tool (step >= 1)", len(all)), totalAll)

	filtered := reg.FilterToolDefs("Xin chào, bạn là ai và làm được gì?", 0)
	totalFiltered := 0
	for _, d := range filtered {
		b, _ := json.Marshal(d)
		totalFiltered += len(b)
	}
	line(fmt.Sprintf("sau filter, chat thường: %d tool", len(filtered)), totalFiltered)

	names := make([]string, 0, len(sizes))
	for n := range sizes {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return sizes[names[i]] > sizes[names[j]] })
	fmt.Println("  — 5 tool schema lớn nhất:")
	for i, n := range names {
		if i >= 5 {
			break
		}
		line("    "+n, sizes[n])
	}

	fmt.Printf("\n═══ Tổng một request chat đơn giản ═══\n")
	sys := base
	if sk := loader.MatchSkill("giải thích giúp tôi RAG là gì cho người mới học"); sk != nil {
		sys += "\n\n[KỸ NĂNG ĐANG KÍCH HOẠT: " + sk.Name + "]\n" + sk.PromptBody()
	}
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "Giải thích ngắn gọn RAG là gì cho người mới."}}
	mb := 0
	for _, m := range msgs {
		mb += len(m.Content)
	}
	line("system (base + skill)", len(sys))
	line("messages", mb)
	line("tool schema (đã filter)", totalFiltered)
	modelCall := len(sys) + mb + totalFiltered
	line("→ lượt gọi MODEL", modelCall)

	// Learner (reflection) chạy nền sau MỖI câu trả lời — nó là một lượt gọi LLM
	// nữa mà người dùng không thấy, nên phải tính vào chi phí một lượt chat.
	fmt.Printf("\n═══ Chi phí ẩn: learner chạy nền mỗi lượt ═══\n")
	reflectionSys := len(memory.ReflectionSystemPromptForSizing())
	// Transcript thực tế: chỉ maxReflectionMessages tin nhắn cuối (đã cắt), lấy
	// một cặp trao đổi cỡ trung bình làm mẫu.
	sampleExchange := "USER: Giải thích ngắn gọn RAG là gì cho người mới.\n\n" +
		"ASSISTANT: " + strings.Repeat("RAG là kỹ thuật cho model tra cứu tài liệu trước khi trả lời. ", 8) + "\n\n"
	line("system prompt reflection", reflectionSys)
	line("transcript (đã cắt còn lượt cuối)", len(sampleExchange))
	learnerCall := reflectionSys + len(sampleExchange)
	line("→ lượt gọi LEARNER", learnerCall)

	fmt.Printf("\n═══ TỔNG INPUT MỘT LƯỢT CHAT ═══\n")
	line("model + learner", modelCall+learnerCall)
	fmt.Printf("\n  Ghi chú: đây là ƯỚC LƯỢNG từ code (≈3,5 byte/token). Số thật lấy\n")
	fmt.Printf("  từ log production sau khi deploy — dòng `engine: run done tokens_in=...`\n")
	fmt.Printf("  giờ đã đếm đúng (trước đây phồng 8-17 lần), và dòng\n")
	fmt.Printf("  `gemini: chi tiết token` cho biết bao nhiêu token được cache (rẻ hơn ~4x).\n\n")
}
