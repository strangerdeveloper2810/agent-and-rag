package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RecentUserMessages trả về nội dung của tối đa `limit` tin nhắn role=user
// gần nhất của 1 tenant (mới nhất trước) — dùng để cá nhân hoá gợi ý mở đầu
// hội thoại (xem agenthttp.SuggestionsHandler). Collection `messages` do
// apps/api sở hữu/ghi (schema TS: apps/api/src/lib/collections.ts MessageDoc);
// agent-go chỉ ĐỌC, cùng pattern đã có với collection `documents`.
func (c *Client) RecentUserMessages(ctx context.Context, tenantID string, limit int) ([]string, error) {
	coll := c.Collection("messages")
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	cur, err := coll.Find(ctx, bson.M{"tenantId": tenantID, "role": "user"}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []string
	for cur.Next(ctx) {
		var doc struct {
			Content string `bson:"content"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		if doc.Content != "" {
			out = append(out, doc.Content)
		}
	}
	return out, cur.Err()
}
