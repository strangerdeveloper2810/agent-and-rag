package mongo

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// testClient trả về Client nối tới MongoDB thật nếu MONGODB_TEST_URI được set,
// ngược lại skip. Các test không cần DB nằm ở nửa dưới file.
func testClient(t *testing.T) *Client {
	t.Helper()
	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Skip("bỏ qua test tích hợp: chưa set MONGODB_TEST_URI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := Connect(ctx, uri, "agent_go_test")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// --- Không cần DB ---

func TestConnect_InvalidURI(t *testing.T) {
	// URI sai scheme → lỗi ngay ở ApplyURI, không chạm mạng.
	if _, err := Connect(context.Background(), "không-phải-uri", "db"); err == nil {
		t.Fatal("Connect với URI hỏng = nil error, want lỗi")
	}
}

func TestConnect_UnreachableHost(t *testing.T) {
	// serverSelectionTimeoutMS nhỏ để fail nhanh thay vì chờ 10s.
	uri := "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=200&connectTimeoutMS=200"

	if _, err := Connect(context.Background(), uri, "db"); err == nil {
		t.Fatal("Connect tới host không tồn tại = nil error, want lỗi ping")
	}
}

// Update/Delete validate id TRƯỚC khi chạm DB → id hỏng phải lỗi ngay,
// không cần kết nối.
func TestTaskRepo_InvalidIDFailsBeforeDB(t *testing.T) {
	repo := NewTaskRepo(&Client{})

	if err := repo.Update(context.Background(), "id-bậy", bson.M{"status": "done"}); err == nil {
		t.Error("Update với id hỏng = nil error, want lỗi")
	}
	if err := repo.Delete(context.Background(), "id-bậy"); err == nil {
		t.Error("Delete với id hỏng = nil error, want lỗi")
	}
}

func TestNewTaskRepo(t *testing.T) {
	c := &Client{}
	repo := NewTaskRepo(c)
	if repo == nil || repo.c != c {
		t.Errorf("NewTaskRepo = %+v", repo)
	}
}

// --- Cần DB thật (skip nếu không có MONGODB_TEST_URI) ---

func TestClient_PingAndCollection(t *testing.T) {
	c := testClient(t)

	if err := c.Ping(); err != nil {
		t.Errorf("Ping: %v", err)
	}
	if col := c.Collection("tasks"); col == nil || col.Name() != "tasks" {
		t.Errorf("Collection = %+v", col)
	}
}

func TestTaskRepo_CRUD(t *testing.T) {
	c := testClient(t)
	repo := NewTaskRepo(c)
	ctx := context.Background()

	// Dọn sạch trước và sau để test chạy lại được.
	cleanup := func() { _, _ = c.Collection("tasks").DeleteMany(ctx, bson.M{"source": "go-test"}) }
	cleanup()
	t.Cleanup(cleanup)

	created, err := repo.Create(ctx, Task{Title: "viết test", Status: "pending", Source: "go-test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID.IsZero() {
		t.Fatal("Create không trả _id")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("Create không set timestamps")
	}

	tasks, err := repo.List(ctx, bson.M{"source": "go-test"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "viết test" {
		t.Fatalf("List = %+v", tasks)
	}

	if err := repo.Update(ctx, created.ID.Hex(), bson.M{"status": "completed"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	tasks, err = repo.List(ctx, bson.M{"source": "go-test"})
	if err != nil {
		t.Fatalf("List sau Update: %v", err)
	}
	if tasks[0].Status != "completed" {
		t.Errorf("status = %q, want completed", tasks[0].Status)
	}

	if err := repo.Delete(ctx, created.ID.Hex()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	tasks, err = repo.List(ctx, bson.M{"source": "go-test"})
	if err != nil {
		t.Fatalf("List sau Delete: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("còn %d task sau khi xoá", len(tasks))
	}
}

func TestTaskRepo_ListEmpty(t *testing.T) {
	c := testClient(t)
	repo := NewTaskRepo(c)

	tasks, err := repo.List(context.Background(), bson.M{"source": "không-tồn-tại-bao-giờ"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("List = %+v, want rỗng", tasks)
	}
}
