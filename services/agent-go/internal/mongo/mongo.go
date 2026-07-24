// Package mongo bọc mongo-driver v2: kết nối + truy cập collection.
// Go truy cập Mongo TRỰC TIẾP cho documents(read)/tasks/memories (xem design mục 4).
package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Client bọc *mongo.Client + database mặc định.
type Client struct {
	client *mongo.Client
	db     *mongo.Database
}

// Connect mở kết nối và ping (fail nhanh nếu không tới được).
func Connect(ctx context.Context, uri, dbName string) (*Client, error) {
	c, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := c.Ping(pingCtx, nil); err != nil {
		return nil, err
	}
	return &Client{client: c, db: c.Database(dbName)}, nil
}

// Collection trả về collection theo tên trong database mặc định.
func (c *Client) Collection(name string) *mongo.Collection {
	return c.db.Collection(name)
}

// Ping kiểm tra kết nối MongoDB còn sống không.
func (c *Client) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.client.Ping(ctx, nil)
}

// Close ngắt kết nối.
func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}
