package agent

import (
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestNewState_NoAttachments(t *testing.T) {
	s := newState(RunInput{
		UserMessage: "hello",
		MaxSteps:    5,
	})
	if s.MaxSteps != 5 {
		t.Fatalf("MaxSteps = %d, want 5", s.MaxSteps)
	}
	if len(s.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(s.Messages))
	}
	msg := s.Messages[0]
	if msg.Role != provider.RoleUser {
		t.Fatalf("Role = %q, want user", msg.Role)
	}
	if msg.Content != "hello" {
		t.Fatalf("Content = %q, want hello", msg.Content)
	}
	if len(msg.Attachments) != 0 {
		t.Fatalf("Attachments = %d, want 0", len(msg.Attachments))
	}
}

func TestNewState_WithFileAttachments(t *testing.T) {
	s := newState(RunInput{
		UserMessage: "analyze this",
		Attachments: []provider.Attachment{
			{Type: "file", Name: "readme.txt", Data: "line1\nline2", MimeType: "text/plain"},
		},
		MaxSteps: 10,
	})
	if len(s.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(s.Messages))
	}
	msg := s.Messages[0]
	if len(msg.Attachments) != 0 {
		t.Fatalf("file attachments should NOT be stored in Attachments: got %d", len(msg.Attachments))
	}
	if !strings.Contains(msg.Content, "[File: readme.txt]") {
		t.Fatalf("Content missing file marker: %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "line1\nline2") {
		t.Fatalf("Content missing file data: %q", msg.Content)
	}
}

func TestNewState_WithImageAttachments(t *testing.T) {
	s := newState(RunInput{
		UserMessage: "what is this?",
		Attachments: []provider.Attachment{
			{Type: "image", Name: "photo.png", Data: "base64data", MimeType: "image/png"},
			{Type: "image", Name: "diagram.jpg", Data: "base64data2", MimeType: "image/jpeg"},
		},
		MaxSteps: 10,
	})
	if len(s.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(s.Messages))
	}
	msg := s.Messages[0]
	if len(msg.Attachments) != 2 {
		t.Fatalf("len(Attachments) = %d, want 2", len(msg.Attachments))
	}
	if msg.Attachments[0].Name != "photo.png" {
		t.Errorf("att[0].Name = %q, want photo.png", msg.Attachments[0].Name)
	}
	if msg.Attachments[1].Name != "diagram.jpg" {
		t.Errorf("att[1].Name = %q, want diagram.jpg", msg.Attachments[1].Name)
	}
	if !strings.Contains(msg.Content, "[Image attached: photo.png]") {
		t.Errorf("Content missing image marker: %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "[Image attached: diagram.jpg]") {
		t.Errorf("Content missing second image marker: %q", msg.Content)
	}
}

func TestNewState_MixedAttachments(t *testing.T) {
	s := newState(RunInput{
		UserMessage: "check these",
		Attachments: []provider.Attachment{
			{Type: "image", Name: "screenshot.png", Data: "imgdata", MimeType: "image/png"},
			{Type: "file", Name: "log.txt", Data: "error: out of memory", MimeType: "text/plain"},
		},
		MaxSteps: 10,
	})
	msg := s.Messages[0]
	// Only image should be in Attachments
	if len(msg.Attachments) != 1 {
		t.Fatalf("len(Attachments) = %d, want 1 (only images)", len(msg.Attachments))
	}
	if msg.Attachments[0].Name != "screenshot.png" {
		t.Errorf("image att not preserved: %q", msg.Attachments[0].Name)
	}
	if !strings.Contains(msg.Content, "[Image attached: screenshot.png]") {
		t.Errorf("Content missing image marker: %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "[File: log.txt]") {
		t.Errorf("Content missing file marker: %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "error: out of memory") {
		t.Errorf("Content missing file data: %q", msg.Content)
	}
}

func TestNewState_DefaultMaxSteps(t *testing.T) {
	s := newState(RunInput{
		UserMessage: "hello",
	})
	if s.MaxSteps != 12 {
		t.Fatalf("default MaxSteps = %d, want 12", s.MaxSteps)
	}
}
