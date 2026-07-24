package http

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestValidateAttachments_Empty(t *testing.T) {
	if err := validateAttachments(nil); err != nil {
		t.Fatalf("nil attachments: got error %v, want nil", err)
	}
	if err := validateAttachments([]Attachment{}); err != nil {
		t.Fatalf("empty attachments: got error %v, want nil", err)
	}
}

func TestValidateAttachments_Valid(t *testing.T) {
	atts := []Attachment{
		{Type: "image", Name: "photo.png", Data: base64.StdEncoding.EncodeToString([]byte("fake-image-data")), MimeType: "image/png"},
		{Type: "file", Name: "readme.txt", Data: "hello world", MimeType: "text/plain"},
	}
	if err := validateAttachments(atts); err != nil {
		t.Fatalf("valid attachments: got error %v, want nil", err)
	}
}

func TestValidateAttachments_TooManyImages(t *testing.T) {
	atts := make([]Attachment, maxImagesPerRequest+1)
	for i := range atts {
		atts[i] = Attachment{
			Type:     "image",
			Name:     fmt.Sprintf("img%d.png", i),
			Data:     base64.StdEncoding.EncodeToString([]byte("x")),
			MimeType: "image/png",
		}
	}
	err := validateAttachments(atts)
	if err == nil {
		t.Fatal("expected error for too many images, got nil")
	}
	if !strings.Contains(err.Error(), "too many images") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttachments_TooManyFiles(t *testing.T) {
	atts := make([]Attachment, maxFilesPerRequest+1)
	for i := range atts {
		atts[i] = Attachment{
			Type:     "file",
			Name:     fmt.Sprintf("doc%d.txt", i),
			Data:     "content",
			MimeType: "text/plain",
		}
	}
	err := validateAttachments(atts)
	if err == nil {
		t.Fatal("expected error for too many files, got nil")
	}
	if !strings.Contains(err.Error(), "too many files") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttachments_ImageTooLarge(t *testing.T) {
	// Create an image whose decoded byte size exceeds 10MB.
	largeData := make([]byte, maxAttachmentBytes+1)
	atts := []Attachment{{
		Type:     "image",
		Name:     "big.jpg",
		Data:     base64.StdEncoding.EncodeToString(largeData),
		MimeType: "image/jpeg",
	}}
	err := validateAttachments(atts)
	if err == nil {
		t.Fatal("expected error for oversized image, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds 10 MB limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttachments_FileTooLarge(t *testing.T) {
	largeData := strings.Repeat("x", maxAttachmentBytes+1)
	atts := []Attachment{{
		Type:     "file",
		Name:     "big.txt",
		Data:     largeData,
		MimeType: "text/plain",
	}}
	err := validateAttachments(atts)
	if err == nil {
		t.Fatal("expected error for oversized file, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds 10 MB limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttachments_UnknownType(t *testing.T) {
	atts := []Attachment{{
		Type:     "video",
		Name:     "clip.mp4",
		Data:     "data",
		MimeType: "video/mp4",
	}}
	err := validateAttachments(atts)
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if !strings.Contains(err.Error(), "unknown attachment type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttachments_InvalidBase64(t *testing.T) {
	atts := []Attachment{{
		Type:     "image",
		Name:     "bad.png",
		Data:     "!!!not-valid-base64!!!",
		MimeType: "image/png",
	}}
	err := validateAttachments(atts)
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
	if !strings.Contains(err.Error(), "invalid base64") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttachments_Mixed(t *testing.T) {
	// Max images + max files + text-only user message = valid.
	atts := make([]Attachment, 0, maxImagesPerRequest+maxFilesPerRequest)
	for i := 0; i < maxImagesPerRequest; i++ {
		atts = append(atts, Attachment{
			Type:     "image",
			Name:     fmt.Sprintf("img%d.png", i),
			Data:     base64.StdEncoding.EncodeToString([]byte("pixel")),
			MimeType: "image/png",
		})
	}
	for i := 0; i < maxFilesPerRequest; i++ {
		atts = append(atts, Attachment{
			Type:     "file",
			Name:     fmt.Sprintf("doc%d.txt", i),
			Data:     "text",
			MimeType: "text/plain",
		})
	}
	if err := validateAttachments(atts); err != nil {
		t.Fatalf("mixed max attachments: got error %v, want nil", err)
	}
}
