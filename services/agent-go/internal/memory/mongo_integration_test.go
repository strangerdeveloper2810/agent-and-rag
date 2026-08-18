package memory

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	agentmongo "github.com/ai-agent-tut/agent-go/internal/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Integration test cho đường lưu Mongo của learner + store.
//
// Ba hàm này (saveFactToMongo, saveKnowledgeItemToMongo, LoadFromMongo) không fake
// được: mongo.Client chỉ dựng qua Connect() có ping ngay lúc tạo, và struct có
// field unexported. Nên phải có MongoDB thật.
//
// Chạy:
//
//	docker run -d --rm --name jarvis-test-mongo -p 27117:27017 mongo:7
//	MONGODB_TEST_URI=mongodb://localhost:27117 go test ./internal/memory/
//
// Không set MONGODB_TEST_URI thì skip — CI không có Mongo vẫn xanh.
func mongoOrSkip(t *testing.T) (*agentmongo.Client, func()) {
	t.Helper()

	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Skip("bỏ qua: cần MONGODB_TEST_URI (xem comment đầu file)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// DB riêng cho mỗi test để các test không nhìn thấy dữ liệu của nhau.
	dbName := "jarvis_test_" + t.Name()
	client, err := agentmongo.Connect(ctx, uri, dbName)
	if err != nil {
		t.Fatalf("không kết nối được Mongo test: %v", err)
	}

	cleanup := func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		_ = client.Collection("memories").Drop(dropCtx)
		_ = client.Collection("documents").Drop(dropCtx)
		_ = client.Close(dropCtx)
	}
	return client, cleanup
}

func TestLearner_LuuFactVaoMongo(t *testing.T) {
	client, cleanup := mongoOrSkip(t)
	defer cleanup()

	prov := &scriptedLearnerProvider{json: `{
		"user_facts": [
			{"category":"tech_stack","key":"backend_framework","value":"Go + Fastify","confidence":0.9}
		],
		"knowledge_items": []
	}`}

	store := NewStore()
	l := NewLearner(store, client, prov, "deepseek-v4-flash", nil)
	l.LearnFromConversation(tenantCtx("tenant-a"),
		exchange("Dự án của tôi dùng Go với Fastify nhé", "Ghi nhận."), "conv-1")

	coll := client.Collection("memories")
	ok := waitFor(5*time.Second, func() bool {
		n, err := coll.CountDocuments(context.Background(), bson.M{"key": "backend_framework"})
		return err == nil && n > 0
	})
	if !ok {
		t.Fatal("fact không được ghi vào Mongo")
	}

	var doc struct {
		Key      string  `bson:"key"`
		Value    string  `bson:"value"`
		TenantID string  `bson:"tenantId"`
		Type     string  `bson:"type"`
		Conf     float64 `bson:"confidence"`
	}
	if err := coll.FindOne(context.Background(), bson.M{"key": "backend_framework"}).Decode(&doc); err != nil {
		t.Fatalf("đọc lại document lỗi: %v", err)
	}

	if doc.Value != "Go + Fastify" {
		t.Errorf("value = %q, want %q", doc.Value, "Go + Fastify")
	}
	// tenantId là thứ quan trọng nhất: thiếu nó thì hai tenant học cùng một key sẽ
	// ghi đè lên nhau.
	if doc.TenantID != "tenant-a" {
		t.Errorf("tenantId = %q, want %q", doc.TenantID, "tenant-a")
	}
	if doc.Type != "tech_stack" {
		t.Errorf("type = %q, want %q", doc.Type, "tech_stack")
	}
}

// Hai tenant học cùng một key PHẢI thành hai document, không ghi đè nhau.
func TestLearner_UpsertKhongTronLanGiuaTenant(t *testing.T) {
	client, cleanup := mongoOrSkip(t)
	defer cleanup()

	newLearner := func(value string) {
		prov := &scriptedLearnerProvider{json: `{
			"user_facts": [{"category":"user_profile","key":"user_name","value":"` + value + `","confidence":0.95}],
			"knowledge_items": []
		}`}
		l := NewLearner(NewStore(), client, prov, "m", nil)
		l.LearnFromConversation(tenantCtx("tenant-"+value),
			exchange("tôi tên "+value, "Chào bạn."), "conv-1")
	}

	newLearner("An")
	newLearner("Binh")

	coll := client.Collection("memories")
	ok := waitFor(5*time.Second, func() bool {
		n, err := coll.CountDocuments(context.Background(), bson.M{"key": "user_name"})
		return err == nil && n >= 2
	})
	if !ok {
		n, _ := coll.CountDocuments(context.Background(), bson.M{"key": "user_name"})
		t.Fatalf("chỉ có %d document cho key user_name, want 2 — hai tenant đang ghi đè nhau", n)
	}
}

func TestLearner_LuuKnowledgeItemVaoMongo(t *testing.T) {
	client, cleanup := mongoOrSkip(t)
	defer cleanup()

	prov := &scriptedLearnerProvider{json: `{
		"user_facts": [],
		"knowledge_items": [
			{"title":"Sửa lỗi timeout khi gọi API","summary":"Tăng timeout và thêm retry","tags":["api","timeout"],"content":"# Vấn đề\nTimeout mặc định quá ngắn."}
		]
	}`}

	l := NewLearner(NewStore(), client, prov, "m", nil)
	// Câu phải đủ dài để qua gate worthLearning (câu ngắn + trả lời ngắn bị coi là
	// tán gẫu và không gọi reflection — xem learner_gate.go).
	l.LearnFromConversation(tenantCtx("tenant-a"),
		exchange("sao gọi API bên thanh toán cứ bị timeout liên tục vậy",
			"Do timeout mặc định quá ngắn, tăng lên và thêm retry có backoff."), "conv-42")

	// Knowledge item được lưu vào collection `documents` (chung với RAG), không
	// phải `knowledge` — xem saveKnowledgeItemToMongo.
	coll := client.Collection("documents")
	ok := waitFor(5*time.Second, func() bool {
		n, err := coll.CountDocuments(context.Background(), bson.M{})
		return err == nil && n > 0
	})
	if !ok {
		t.Fatal("knowledge item không được ghi vào Mongo")
	}

	// Knowledge item được lưu dưới dạng document RAG: documentId + text, để
	// rag.search đọc lại được. Không có field `title` riêng.
	var doc struct {
		DocumentID string `bson:"documentId"`
		Source     string `bson:"source"`
		Text       string `bson:"text"`
		TenantID   string `bson:"tenantId"`
	}
	if err := coll.FindOne(context.Background(), bson.M{}).Decode(&doc); err != nil {
		t.Fatalf("đọc lại knowledge item lỗi: %v", err)
	}
	if doc.DocumentID != "learned-s-a-l-i-timeout-khi-g-i-api" {
		t.Logf("documentId = %q (slug hoá từ tiêu đề tiếng Việt)", doc.DocumentID)
	}
	if !strings.HasPrefix(doc.DocumentID, "learned-") {
		t.Errorf("documentId = %q, phải có tiền tố learned-", doc.DocumentID)
	}
	if !strings.Contains(doc.Text, "Sửa lỗi timeout khi gọi API") {
		t.Errorf("text không chứa tiêu đề knowledge item: %q", doc.Text)
	}
	if !strings.Contains(doc.Text, "timeout") {
		t.Error("text mất phần nội dung")
	}
	if doc.TenantID != "tenant-a" {
		t.Errorf("tenantId = %q, want tenant-a", doc.TenantID)
	}
}

func TestStore_LoadFromMongo(t *testing.T) {
	client, cleanup := mongoOrSkip(t)
	defer cleanup()

	ctx := context.Background()
	coll := client.Collection("memories")
	if _, err := coll.InsertMany(ctx, []any{
		bson.M{"key": "user_name", "value": "An", "tenantId": "tenant-a", "type": "user_profile"},
		bson.M{"key": "framework", "value": "Fastify", "tenantId": "tenant-a", "type": "tech_stack"},
		bson.M{"key": "user_name", "value": "Binh", "tenantId": "tenant-b", "type": "user_profile"},
	}); err != nil {
		t.Fatalf("seed dữ liệu lỗi: %v", err)
	}

	store := NewStore()
	n, err := store.LoadFromMongo(ctx, client)
	if err != nil {
		t.Fatalf("LoadFromMongo lỗi: %v", err)
	}
	if n != 3 {
		t.Errorf("nạp %d item, want 3", n)
	}

	if v, ok := store.Get("tenant-a", "user_name"); !ok || v != "An" {
		t.Errorf("tenant-a/user_name = %q (%v), want An", v, ok)
	}
	// Nạp phải giữ đúng scope tenant, không trộn.
	if v, ok := store.Get("tenant-b", "user_name"); !ok || v != "Binh" {
		t.Errorf("tenant-b/user_name = %q (%v), want Binh", v, ok)
	}
	if _, ok := store.Get("tenant-b", "framework"); ok {
		t.Error("tenant-b thấy được fact của tenant-a sau khi load — rò rỉ giữa tenant")
	}
}

func TestStore_LoadFromMongo_ClientNil(t *testing.T) {
	// Không có Mongo thì trả 0, không lỗi — server vẫn chạy được ở chế độ in-memory.
	n, err := NewStore().LoadFromMongo(context.Background(), nil)
	if err != nil {
		t.Errorf("LoadFromMongo(nil) lỗi: %v", err)
	}
	if n != 0 {
		t.Errorf("nạp %d item với client nil, want 0", n)
	}
}
