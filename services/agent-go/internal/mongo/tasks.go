package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TaskRepo CRUD collection `tasks` (dùng cho task tools của agent).
type TaskRepo struct{ c *Client }

func NewTaskRepo(c *Client) *TaskRepo { return &TaskRepo{c: c} }

// Create chèn task mới (set timestamps), trả về task kèm _id.
func (r *TaskRepo) Create(ctx context.Context, t Task) (Task, error) {
	now := time.Now()
	t.CreatedAt, t.UpdatedAt = now, now
	res, err := r.c.Collection("tasks").InsertOne(ctx, t)
	if err != nil {
		return Task{}, err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		t.ID = oid
	}
	return t, nil
}

// List lấy task theo filter (bson.M{} = tất cả), mới nhất trước.
func (r *TaskRepo) List(ctx context.Context, filter bson.M) ([]Task, error) {
	cur, err := r.c.Collection("tasks").Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	if err := cur.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// Update cập nhật task theo id (validate id trước khi chạm DB).
func (r *TaskRepo) Update(ctx context.Context, id string, patch bson.M) error {
	oid, err := ToObjectID(id)
	if err != nil {
		return err
	}
	patch["updatedAt"] = time.Now()
	_, err = r.c.Collection("tasks").UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": patch})
	return err
}

// Delete xoá task theo id.
func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	oid, err := ToObjectID(id)
	if err != nil {
		return err
	}
	_, err = r.c.Collection("tasks").DeleteOne(ctx, bson.M{"_id": oid})
	return err
}
