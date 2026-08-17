package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// rag.list tồn tại vì rag.search KHÔNG THỂ bảo đảm liệt kê đủ: nó là tìm kiếm
// ngữ nghĩa top-K. Log dev thật cho thấy khi người dùng hỏi "trong knowledge
// base có gì", agent gọi `echo` với "list all rag documents" rồi brute-force 4
// lần rag.search mà vẫn thiếu tài liệu.

// matchStage trích $match ra khỏi pipeline để assert.
func matchStage(t *testing.T, pipeline []bson.D) bson.D {
	t.Helper()
	for _, stage := range pipeline {
		for _, e := range stage {
			if e.Key == "$match" {
				m, ok := e.Value.(bson.D)
				if !ok {
					t.Fatalf("$match không phải bson.D: %T", e.Value)
				}
				return m
			}
		}
	}
	t.Fatal("pipeline thiếu stage $match")
	return nil
}

// Thiếu clause tenantId ở đây nghĩa là mọi user liệt kê được tài liệu của user
// khác — cùng lớp lỗi đã từng có ở rag.read và notes.search.
func TestBuildRAGListPipeline_ScopesToTenant(t *testing.T) {
	m := matchStage(t, buildRAGListPipeline("tenant-abc"))

	var found bool
	for _, e := range m {
		if e.Key == "tenantId" {
			found = true
			if e.Value != "tenant-abc" {
				t.Errorf("tenantId = %v, want tenant-abc", e.Value)
			}
		}
	}
	if !found {
		t.Errorf("$match thiếu tenantId → rò rỉ danh sách tài liệu giữa các user: %+v", m)
	}
}

// tenant "default"/rỗng = chế độ single-user (không có header X-Tenant-ID) →
// không lọc, giữ tương thích với dữ liệu cũ chưa có tenantId.
func TestBuildRAGListPipeline_DefaultTenantNoFilter(t *testing.T) {
	for _, tenant := range []string{"", "default"} {
		m := matchStage(t, buildRAGListPipeline(tenant))
		for _, e := range m {
			if e.Key == "tenantId" {
				t.Errorf("tenant %q không nên sinh filter tenantId, nhưng có: %v", tenant, e.Value)
			}
		}
	}
}

// Pipeline phải GOM theo documentId — collection lưu theo chunk, nếu không gom
// thì một tài liệu 20 chunk sẽ hiện thành 20 "tài liệu".
func TestBuildRAGListPipeline_GroupsByDocument(t *testing.T) {
	pipeline := buildRAGListPipeline("t1")

	var hasGroup, hasLimit, hasSort bool
	for _, stage := range pipeline {
		for _, e := range stage {
			switch e.Key {
			case "$group":
				g, ok := e.Value.(bson.D)
				if !ok {
					t.Fatalf("$group không phải bson.D: %T", e.Value)
				}
				for _, ge := range g {
					if ge.Key == "_id" && ge.Value != "$documentId" {
						t.Errorf("$group._id = %v, want $documentId", ge.Value)
					}
				}
				hasGroup = true
			case "$limit":
				hasLimit = true
			case "$sort":
				hasSort = true
			}
		}
	}
	if !hasGroup {
		t.Error("pipeline thiếu $group → mỗi chunk sẽ thành 1 tài liệu")
	}
	if !hasLimit {
		t.Error("pipeline thiếu $limit → knowledge base lớn sẽ tạo output khổng lồ")
	}
	if !hasSort {
		t.Error("pipeline thiếu $sort → thứ tự không ổn định giữa các lần gọi")
	}
}

// Không cấu hình Mongo → trả thông báo êm, không lỗi (giống rag.search/rag.read).
func TestRAGListTool_NoMongoIsGraceful(t *testing.T) {
	tool := NewRAGListTool(nil, "db")

	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("không nên trả lỗi khi thiếu Mongo: %v", err)
	}
	if !strings.Contains(res.Content, "RAG not configured") {
		t.Errorf("Content = %q, want thông báo RAG chưa cấu hình", res.Content)
	}
}

// Schema không có tham số bắt buộc — model chỉ cần gọi rag.list là xong. Nếu có
// tham số bắt buộc thì sẽ lặp lại đúng lỗi của notes.search ("query is required"
// khi model muốn liệt kê tất cả).
func TestRAGListTool_SchemaHasNoRequiredArgs(t *testing.T) {
	tool := NewRAGListTool(nil, "db")

	var schema map[string]any
	if err := json.Unmarshal(tool.Schema(), &schema); err != nil {
		t.Fatalf("schema không parse được: %v", err)
	}
	if req, ok := schema["required"]; ok {
		t.Errorf("schema có 'required' = %v, want không có tham số bắt buộc", req)
	}
	if tool.Name() != "rag.list" {
		t.Errorf("Name = %q, want rag.list", tool.Name())
	}
	if tool.Kind() != KindRead {
		t.Errorf("Kind = %v, want KindRead (chỉ đọc metadata)", tool.Kind())
	}
	// Description phải nói rõ khi nào dùng, nếu không model vẫn đi brute-force
	// rag.search như trong log.
	desc := tool.Description()
	for _, want := range []string{"ALL", "rag.search"} {
		if !strings.Contains(desc, want) {
			t.Errorf("Description thiếu %q: %s", want, desc)
		}
	}
}
