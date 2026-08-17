package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Bug: rag.read trước fix build match filter CHỈ từ documentId/source, không lọc
// theo tenantId — bất kỳ tenant nào biết/đoán được documentId hoặc source của
// tenant khác đều đọc được toàn bộ nội dung tài liệu đó. buildRAGReadFilter là
// hàm thuần chịu trách nhiệm build filter này (không cần Mongo thật để test).

func filterHasTenant(t *testing.T, filter bson.D, wantTenant string) {
	t.Helper()
	for _, e := range filter {
		if e.Key == "tenantId" {
			if e.Value != wantTenant {
				t.Errorf("tenantId trong filter = %v, want %q", e.Value, wantTenant)
			}
			return
		}
	}
	t.Errorf("filter thiếu tenantId=%q: %+v", wantTenant, filter)
}

func filterHasNoTenant(t *testing.T, filter bson.D) {
	t.Helper()
	for _, e := range filter {
		if e.Key == "tenantId" {
			t.Errorf("filter không nên có tenantId, nhưng có: %v (%+v)", e.Value, filter)
		}
	}
}

func TestBuildRAGReadFilter_ScopesToTenant(t *testing.T) {
	// Tenant thật (không phải "default") → filter PHẢI có tenantId, để tenant
	// khác không đọc chéo được document dù biết đúng documentId/source.
	filter := buildRAGReadFilter("doc-123", "", "tenant-a")
	filterHasTenant(t, filter, "tenant-a")

	filter = buildRAGReadFilter("", "go-language.md", "tenant-b")
	filterHasTenant(t, filter, "tenant-b")

	// Hai tenant khác nhau cùng documentId → filter khác nhau, không thể đọc chéo.
	filterA := buildRAGReadFilter("shared-doc-id", "", "tenant-a")
	filterB := buildRAGReadFilter("shared-doc-id", "", "tenant-b")
	filterHasTenant(t, filterA, "tenant-a")
	filterHasTenant(t, filterB, "tenant-b")
}

func TestBuildRAGReadFilter_DefaultTenantNotScoped(t *testing.T) {
	// tenantID rỗng hoặc "default" → giữ nguyên hành vi cũ (không multi-tenant),
	// giống cách ragSearchTool.vectorSearch/textSearch đã làm.
	filterHasNoTenant(t, buildRAGReadFilter("doc-123", "", ""))
	filterHasNoTenant(t, buildRAGReadFilter("doc-123", "", "default"))
}

func TestBuildRAGReadFilter_DocumentIDTakesPrecedenceOverSource(t *testing.T) {
	filter := buildRAGReadFilter("doc-123", "ignored.md", "tenant-a")
	var gotKey string
	for _, e := range filter {
		if e.Key == "documentId" || e.Key == "source" {
			gotKey = e.Key
		}
	}
	if gotKey != "documentId" {
		t.Errorf("khi có cả documentId và source, filter nên ưu tiên documentId, got key = %q", gotKey)
	}
}

// End-to-end mức tool: chưa cấu hình Mongo vẫn phải degrade êm dù request có
// tenant trong context (không panic, không lộ lỗi nội bộ).
func TestRAGReadTool_NotConfigured_WithTenantContext(t *testing.T) {
	tool := NewRAGReadTool(nil, "db")

	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-a")
	res, err := tool.Execute(ctx, json.RawMessage(`{"documentId":"doc-1"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Content, "RAG not configured") {
		t.Errorf("Content = %q", res.Content)
	}
}

func TestRAGReadTool_NotConfigured_TakesPrecedenceOverArgValidation(t *testing.T) {
	tool := NewRAGReadTool(nil, "db")
	// mongoClient nil → degrade êm ngay, kể cả khi args rỗng (chưa tới bước
	// kiểm tra documentId/source required). Giữ nguyên hành vi cũ.
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Content, "RAG not configured") {
		t.Errorf("Content = %q", res.Content)
	}
}

func TestRAGReadTool_Metadata(t *testing.T) {
	tool := NewRAGReadTool(nil, "db")
	if tool.Name() != "rag.read" {
		t.Errorf("Name() = %q, want rag.read", tool.Name())
	}
	if tool.Kind() != KindRead {
		t.Errorf("Kind() = %d, want KindRead", tool.Kind())
	}
}
