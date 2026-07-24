// Package provider — FakeProvider để test engine deterministic, không cần mạng.
package provider

import "context"

// FakeProvider trả về danh sách chunk được lập trình sẵn.
// Dùng trong test engine/model/tools/loop — không gọi LLM thật.
type FakeProvider struct {
	chunks []StreamChunk
}

// NewFake tạo FakeProvider với kịch bản chunk cho trước.
// chunks được gửi theo thứ tự vào channel, sau đó channel được đóng.
func NewFake(chunks ...StreamChunk) *FakeProvider {
	return &FakeProvider{chunks: chunks}
}

// Name trả về "fake".
func (f *FakeProvider) Name() string { return "fake" }

// Generate trả về channel chứa các chunk đã lập trình, rồi đóng.
// Tôn trọng ctx: nếu ctx bị cancel giữa chừng, dừng gửi và return.
func (f *FakeProvider) Generate(ctx context.Context, _ GenerateRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, len(f.chunks)+1)

	go func() {
		defer close(ch)
		for _, c := range f.chunks {
			select {
			case ch <- c:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}
