package tools

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestCalculatorTool(t *testing.T) {
	tool := NewCalculatorTool()

	tests := []struct {
		name       string
		expression string
		wantResult float64
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "simple addition",
			expression: "2+2",
			wantResult: 4,
		},
		{
			name:       "multiplication",
			expression: "3*4",
			wantResult: 12,
		},
		{
			name:       "division",
			expression: "10/2",
			wantResult: 5,
		},
		{
			name:       "subtraction",
			expression: "10-7",
			wantResult: 3,
		},
		{
			name:       "power with double star",
			expression: "2**3",
			wantResult: 8,
		},
		{
			name:       "power with caret",
			expression: "2^3",
			wantResult: 8,
		},
		{
			name:       "sqrt function",
			expression: "sqrt(16)",
			wantResult: 4,
		},
		{
			name:       "sin function",
			expression: "sin(0)",
			wantResult: 0,
		},
		{
			name:       "cos function",
			expression: "cos(0)",
			wantResult: 1,
		},
		{
			name:       "abs positive",
			expression: "abs(5)",
			wantResult: 5,
		},
		{
			name:       "abs negative",
			expression: "abs(-5)",
			wantResult: 5,
		},
		{
			name:       "round down",
			expression: "round(3.2)",
			wantResult: 3,
		},
		{
			name:       "round up",
			expression: "round(3.7)",
			wantResult: 4,
		},
		{
			name:       "floor",
			expression: "floor(3.9)",
			wantResult: 3,
		},
		{
			name:       "ceil",
			expression: "ceil(3.1)",
			wantResult: 4,
		},
		{
			name:       "parentheses grouping",
			expression: "(2+3)*4",
			wantResult: 20,
		},
		{
			name:       "operator precedence",
			expression: "2+3*4",
			wantResult: 14,
		},
		{
			name:       "complex expression",
			expression: "2+3*4-8/2",
			wantResult: 10,
		},
		{
			name:       "unary minus",
			expression: "-5+3",
			wantResult: -2,
		},
		{
			name:       "log function",
			expression: "log(1)",
			wantResult: 0,
		},
		{
			name:       "exp function",
			expression: "exp(0)",
			wantResult: 1,
		},
		{
			name:       "modulo",
			expression: "10%3",
			wantResult: 1,
		},
		{
			name:       "tan function",
			expression: "tan(0)",
			wantResult: 0,
		},
		{
			name:       "pi constant",
			expression: "pi",
			wantResult: math.Pi,
		},
		{
			name:       "e constant",
			expression: "e",
			wantResult: math.E,
		},
		{
			name:       "nested parentheses",
			expression: "((2+3)*(4+1))",
			wantResult: 25,
		},
		{
			name:       "nested functions",
			expression: "sqrt(abs(-25))",
			wantResult: 5,
		},
		{
			name:       "whitespace handling",
			expression: " 2 + 3 *  4 ",
			wantResult: 14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"expression": tt.expression})
			res, err := tool.Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var out struct {
				Expression string  `json:"expression"`
				Result     float64 `json:"result"`
			}
			if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}

			if out.Expression != tt.expression {
				t.Errorf("expression: got %q, want %q", out.Expression, tt.expression)
			}
			// Use tolerance for floating point comparison
			if math.Abs(out.Result-tt.wantResult) > 1e-9 {
				t.Errorf("result: got %v, want %v", out.Result, tt.wantResult)
			}
		})
	}
}

func TestCalculatorTool_Errors(t *testing.T) {
	tool := NewCalculatorTool()

	tests := []struct {
		name       string
		expression string
		errMsg     string
	}{
		{
			name:       "empty expression",
			expression: "",
			errMsg:     "expression is required",
		},
		{
			name:       "whitespace only",
			expression: "   ",
			errMsg:     "expression is required",
		},
		{
			name:       "division by zero",
			expression: "1/0",
			errMsg:     "division by zero",
		},
		{
			name:       "unknown function",
			expression: "foo(5)",
			errMsg:     "unknown function",
		},
		{
			name:       "invalid syntax",
			expression: "2*/3",
			errMsg:     "unexpected",
		},
		{
			name:       "sqrt of negative",
			expression: "sqrt(-1)",
			errMsg:     "sqrt of negative",
		},
		{
			name:       "unmatched paren",
			expression: "(2+3",
			errMsg:     "missing closing parenthesis",
		},
		{
			name:       "log of zero",
			expression: "log(0)",
			errMsg:     "log of non-positive",
		},
		{
			name:       "log of negative",
			expression: "log(-1)",
			errMsg:     "log of non-positive",
		},
		{
			name:       "function without parens",
			expression: "missing",
			errMsg:     "expected '(' after function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"expression": tt.expression})
			_, err := tool.Execute(context.Background(), args)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestCalculatorTool_InvalidArgs(t *testing.T) {
	tool := NewCalculatorTool()

	t.Run("missing expression param", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing expression, got nil")
		}
		if !strings.Contains(err.Error(), "expression is required") {
			t.Errorf("expected 'expression is required' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "invalid args") {
			t.Errorf("expected 'invalid args' error, got: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		args, _ := json.Marshal(map[string]string{"expression": "2+2"})
		_, err := tool.Execute(ctx, args)
		if err == nil {
			t.Fatal("expected error for cancelled context, got nil")
		}
	})
}

func TestCalculatorToolInterface(t *testing.T) {
	tool := NewCalculatorTool()
	if tool.Name() != "calculator" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "calculator")
	}
	if tool.Kind() != KindRead {
		t.Errorf("Kind: got %v, want KindRead", tool.Kind())
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema is empty")
	}

	// Verify schema contains required fields
	var schema map[string]any
	json.Unmarshal(tool.Schema(), &schema)
	if schema["type"] != "object" {
		t.Error("Schema type should be object")
	}
	required, ok := schema["required"].([]any)
	if !ok || len(required) == 0 {
		t.Error("Schema should have required fields")
	}
}
