package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLoader(t *testing.T) {
	dir := t.TempDir()

	// Create skills/code-review/SKILL.md
	codeReviewDir := filepath.Join(dir, "code-review")
	mustMkdir(t, codeReviewDir)
	mustWriteFile(t, filepath.Join(codeReviewDir, "SKILL.md"), `---
name: code-review
description: Review code for bugs
when_to_use: When user asks for code review
tools: [file.read, git.diff]
---

# Code Review
Check the code carefully.`)

	// Create skills/debug/SKILL.md
	debugDir := filepath.Join(dir, "debug")
	mustMkdir(t, debugDir)
	mustWriteFile(t, filepath.Join(debugDir, "SKILL.md"), `---
name: debug
description: Debug systematically
when_to_use: When user reports a bug or error
tools: [shell.exec, file.read]
---

# Debug
Reproduce then isolate then fix.`)

	loader, err := NewLoader(dir)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	if loader.Len() != 2 {
		t.Errorf("expected 2 skills, got %d", loader.Len())
	}

	// Verify code-review loaded correctly
	cr := loader.LoadSkill("code-review")
	if cr == nil {
		t.Fatal("code-review skill not found")
	}
	if cr.Name != "code-review" {
		t.Errorf("name = %q, want %q", cr.Name, "code-review")
	}
	if cr.Description != "Review code for bugs" {
		t.Errorf("description = %q, want %q", cr.Description, "Review code for bugs")
	}
	if cr.WhenToUse != "When user asks for code review" {
		t.Errorf("when_to_use = %q", cr.WhenToUse)
	}
	if len(cr.Tools) != 2 || cr.Tools[0] != "file.read" || cr.Tools[1] != "git.diff" {
		t.Errorf("tools = %v, want [file.read git.diff]", cr.Tools)
	}
	if !strings.Contains(cr.Content, "# Code Review") {
		t.Errorf("content should contain markdown body")
	}

	// Verify debug loaded correctly
	dbg := loader.LoadSkill("debug")
	if dbg == nil {
		t.Fatal("debug skill not found")
	}
	if dbg.Name != "debug" {
		t.Errorf("name = %q, want %q", dbg.Name, "debug")
	}
}

func TestNewLoader_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	loader, err := NewLoader(dir)
	if err != nil {
		t.Fatalf("NewLoader on empty dir: %v", err)
	}
	if loader.Len() != 0 {
		t.Errorf("expected 0 skills, got %d", loader.Len())
	}
}

func TestNewLoader_DirWithNoSkills(t *testing.T) {
	dir := t.TempDir()
	// Create a directory but no SKILL.md inside it
	noSkillDir := filepath.Join(dir, "empty-skill")
	mustMkdir(t, noSkillDir)

	loader, err := NewLoader(dir)
	if err != nil {
		t.Fatalf("NewLoader on dir with no skills: %v", err)
	}
	if loader.Len() != 0 {
		t.Errorf("expected 0 skills, got %d", loader.Len())
	}
}

func TestNewLoader_NonExistentDir(t *testing.T) {
	_, err := NewLoader("/nonexistent/path/should/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent dir")
	}
}

func TestNewLoader_BadFrontmatter(t *testing.T) {
	dir := t.TempDir()

	badDir := filepath.Join(dir, "bad-skill")
	mustMkdir(t, badDir)
	mustWriteFile(t, filepath.Join(badDir, "SKILL.md"), `no frontmatter here
just some text`)

	_, err := NewLoader(dir)
	if err == nil {
		t.Fatal("expected error for SKILL.md without frontmatter")
	}
}

func TestNewLoader_MissingName(t *testing.T) {
	dir := t.TempDir()

	badDir := filepath.Join(dir, "no-name")
	mustMkdir(t, badDir)
	mustWriteFile(t, filepath.Join(badDir, "SKILL.md"), `---
description: Some description
when_to_use: Some condition
---

# No Name Skill`)

	_, err := NewLoader(dir)
	if err == nil {
		t.Fatal("expected error for SKILL.md without 'name' field")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention missing 'name' field: %v", err)
	}
}

func TestListSkills(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	mustMkdir(t, skillDir)
	mustWriteFile(t, filepath.Join(skillDir, "SKILL.md"), `---
name: test-skill
description: A test skill
when_to_use: For testing
tools: [tool.a]
---

# Test`)

	loader, err := NewLoader(dir)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	summaries := loader.ListSkills()
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Name != "test-skill" {
		t.Errorf("name = %q", summaries[0].Name)
	}
	if summaries[0].Description != "A test skill" {
		t.Errorf("description = %q", summaries[0].Description)
	}
}

func TestLoadSkill_NotFound(t *testing.T) {
	loader := &Loader{skills: make(map[string]*Skill)}
	if skill := loader.LoadSkill("nonexistent"); skill != nil {
		t.Errorf("expected nil for nonexistent skill, got %v", skill)
	}
}

func TestMatchSkill_ByName(t *testing.T) {
	loader := setupTestLoader(t)

	// Exact match with hyphen
	skill := loader.MatchSkill("can you do a code-review of this file?")
	if skill == nil {
		t.Fatal("expected match for 'code-review'")
	}
	if skill.Name != "code-review" {
		t.Errorf("matched wrong skill: %q", skill.Name)
	}

	// Match with space instead of hyphen
	skill = loader.MatchSkill("I need a code review please")
	if skill == nil {
		t.Fatal("expected match for 'code review'")
	}
	if skill.Name != "code-review" {
		t.Errorf("matched wrong skill: %q", skill.Name)
	}
}

func TestMatchSkill_ByWhenToUse(t *testing.T) {
	loader := setupTestLoader(t)

	skill := loader.MatchSkill("I found a bug in the login page")
	if skill == nil {
		t.Fatal("expected match for 'bug' keyword")
	}
	if skill.Name != "debug" {
		t.Errorf("matched wrong skill: %q, want 'debug'", skill.Name)
	}
}

func TestMatchSkill_ByDescription(t *testing.T) {
	loader := setupTestLoader(t)

	skill := loader.MatchSkill("can you help me debug this crash?")
	if skill == nil {
		t.Fatal("expected match for 'debug' keyword")
	}
	if skill.Name != "debug" {
		t.Errorf("matched wrong skill: %q, want 'debug'", skill.Name)
	}
}

func TestMatchSkill_NoMatch(t *testing.T) {
	loader := setupTestLoader(t)

	skill := loader.MatchSkill("tell me a joke")
	if skill != nil {
		t.Errorf("expected nil for unrelated input, got %q", skill.Name)
	}
}

func TestMatchSkill_NameTakesPrecedence(t *testing.T) {
	loader := setupTestLoader(t)

	// "debug" appears in code-review's description, but "code review" in input
	// should match code-review by name first
	skill := loader.MatchSkill("please do a code review with debugging help")
	if skill == nil {
		t.Fatal("expected match")
	}
	if skill.Name != "code-review" {
		t.Errorf("name match should take precedence, got %q", skill.Name)
	}
}

func TestParseToolsList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"[a, b, c]", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{"[]", nil},
		{"", nil},
		{"[only-one]", []string{"only-one"}},
	}

	for _, tt := range tests {
		result := parseToolsList(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseToolsList(%q) len = %d, want %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("parseToolsList(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestContainsAnyKeyword(t *testing.T) {
	tests := []struct {
		text      string
		reference string
		want      bool
	}{
		{"I have a bug in my code", "When user reports a bug or error", true},
		{"let me review this", "When user asks for code review", true},
		{"hi there", "When user asks for code review", false}, // "there" is >= 4 chars but not in text
		{"let's code", "When user asks for code review", true}, // "code" >= 4 chars
		{"do it", "some instructions here", false},             // no reference word appears in text
		{"I have a bug", "When user reports a bug or error", true}, // "bug" >= 3 chars (was filtered at >=4)
	}

	for _, tt := range tests {
		got := containsAnyKeyword(tt.text, tt.reference)
		if got != tt.want {
			t.Errorf("containsAnyKeyword(%q, %q) = %v, want %v", tt.text, tt.reference, got, tt.want)
		}
	}
}

// --- helpers ---

func setupTestLoader(t *testing.T) *Loader {
	t.Helper()

	dir := t.TempDir()

	// code-review skill
	crDir := filepath.Join(dir, "code-review")
	mustMkdir(t, crDir)
	mustWriteFile(t, filepath.Join(crDir, "SKILL.md"), `---
name: code-review
description: Review code for bugs, security, and best practices
when_to_use: When user asks for code review, PR review, or code quality check
tools: [file.read, shell.exec, git.diff, git.log]
---

# Code Review Skill
Review code systematically.`)

	// debug skill
	dbgDir := filepath.Join(dir, "debug")
	mustMkdir(t, dbgDir)
	mustWriteFile(t, filepath.Join(dbgDir, "SKILL.md"), `---
name: debug
description: Systematic debugging reproduce isolate identify fix verify
when_to_use: When user reports a bug, error, crash, or unexpected behavior
tools: [shell.exec, file.read, git.log, git.diff]
---

# Debugging Skill
Debug step by step.`)

	loader, err := NewLoader(dir)
	if err != nil {
		t.Fatalf("setupTestLoader: %v", err)
	}
	return loader
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
