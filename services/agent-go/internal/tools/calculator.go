package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// calculatorTool evaluates math expressions safely (no eval).
type calculatorTool struct{}

// NewCalculatorTool creates a calculator tool.
func NewCalculatorTool() Tool {
	return &calculatorTool{}
}

func (t *calculatorTool) Name() string { return "calculator" }

func (t *calculatorTool) Description() string {
	return "Safely evaluate math expressions. Supports: +, -, *, /, %, **, sqrt, sin, cos, abs, round, floor, ceil, log, log10, exp. Use parentheses for grouping."
}

func (t *calculatorTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"expression":{"type":"string","description":"math expression to evaluate"}
		},
		"required":["expression"],
		"additionalProperties":false
	}`)
}

func (t *calculatorTool) Kind() Kind { return KindRead }

func (t *calculatorTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("calculator: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Expression) == "" {
		return Result{}, fmt.Errorf("calculator: expression is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := evaluate(args.Expression)
	if err != nil {
		return Result{}, fmt.Errorf("calculator: %w", err)
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"expression": args.Expression,
		"result":     result,
	})
	return Result{Content: string(out)}, nil
}

// evaluate is a safe recursive-descent expression parser. No eval of arbitrary code.
func evaluate(expr string) (float64, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return 0, err
	}
	p := &parser{tokens: tokens, pos: 0}
	result, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.tokens) {
		return 0, fmt.Errorf("unexpected token: %q", p.tokens[p.pos])
	}
	return result, nil
}

type tokenKind int

const (
	tokNumber tokenKind = iota
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokPercent
	tokCaret   // ** (power)
	tokLParen
	tokRParen
	tokFunc    // function name
	tokComma
)

type token struct {
	kind  tokenKind
	value string
}

func tokenize(expr string) ([]token, error) {
	var tokens []token
	i := 0
	s := strings.TrimSpace(expr)

	for i < len(s) {
		ch := s[i]

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' {
			i++
			continue
		}

		// Numbers
		if (ch >= '0' && ch <= '9') || ch == '.' {
			j := i
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == '.' || s[j] == 'e' || s[j] == 'E' || s[j] == '+' || s[j] == '-') {
				if (s[j] == '+' || s[j] == '-') && j > i && s[j-1] != 'e' && s[j-1] != 'E' {
					break
				}
				j++
			}
			tokens = append(tokens, token{kind: tokNumber, value: s[i:j]})
			i = j
			continue
		}

		// Functions or identifiers
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			j := i
			for j < len(s) && ((s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= '0' && s[j] <= '9') || s[j] == '_') {
				j++
			}
			name := strings.ToLower(s[i:j])
			switch name {
			case "pi":
				tokens = append(tokens, token{kind: tokNumber, value: fmt.Sprintf("%.15f", math.Pi)})
			case "e":
				tokens = append(tokens, token{kind: tokNumber, value: fmt.Sprintf("%.15f", math.E)})
			default:
				tokens = append(tokens, token{kind: tokFunc, value: name})
			}
			i = j
			continue
		}

		// Operators
		switch ch {
		case '+':
			tokens = append(tokens, token{kind: tokPlus})
		case '-':
			tokens = append(tokens, token{kind: tokMinus})
		case '*':
			if i+1 < len(s) && s[i+1] == '*' {
				tokens = append(tokens, token{kind: tokCaret})
				i += 2
				continue
			}
			tokens = append(tokens, token{kind: tokStar})
		case '/':
			tokens = append(tokens, token{kind: tokSlash})
		case '%':
			tokens = append(tokens, token{kind: tokPercent})
		case '(':
			tokens = append(tokens, token{kind: tokLParen})
		case ')':
			tokens = append(tokens, token{kind: tokRParen})
		case ',':
			tokens = append(tokens, token{kind: tokComma})
		case '^':
			tokens = append(tokens, token{kind: tokCaret})
		default:
			return nil, fmt.Errorf("unexpected character: %q", string(ch))
		}
		i++
	}

	return tokens, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() (token, bool) {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos], true
	}
	return token{}, false
}

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *parser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}

	for {
		tok, ok := p.peek()
		if !ok {
			break
		}
		if tok.kind != tokPlus && tok.kind != tokMinus {
			break
		}
		p.advance()

		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}

		if tok.kind == tokPlus {
			left += right
		} else {
			left -= right
		}
	}

	return left, nil
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseExponent()
	if err != nil {
		return 0, err
	}

	for {
		tok, ok := p.peek()
		if !ok {
			break
		}
		if tok.kind != tokStar && tok.kind != tokSlash && tok.kind != tokPercent {
			break
		}
		p.advance()

		right, err := p.parseExponent()
		if err != nil {
			return 0, err
		}

		switch tok.kind {
		case tokStar:
			left *= right
		case tokSlash:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case tokPercent:
			left = float64(int64(left) % int64(right))
		}
	}

	return left, nil
}

// parseExponent handles the power operator (** or ^) with right-associativity.
func (p *parser) parseExponent() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}

	tok, ok := p.peek()
	if !ok || tok.kind != tokCaret {
		return left, nil
	}
	p.advance()

	// Right-associative: parse the exponent first, then compute power
	right, err := p.parseExponent()
	if err != nil {
		return 0, err
	}

	return math.Pow(left, right), nil
}

func (p *parser) parseFactor() (float64, error) {
	tok, ok := p.peek()
	if !ok {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Unary minus
	if tok.kind == tokMinus {
		p.advance()
		val, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}

	// Unary plus
	if tok.kind == tokPlus {
		p.advance()
		return p.parseFactor()
	}

	// Parenthesized expression
	if tok.kind == tokLParen {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		tok2, ok2 := p.peek()
		if !ok2 || tok2.kind != tokRParen {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.advance()
		return val, nil
	}

	// Function call
	if tok.kind == tokFunc {
		name := tok.value
		p.advance()

		// Must be followed by (
		tok2, ok2 := p.peek()
		if !ok2 || tok2.kind != tokLParen {
			return 0, fmt.Errorf("expected '(' after function %q", name)
		}
		p.advance() // consume (

		arg, err := p.parseExpression()
		if err != nil {
			return 0, err
		}

		// Handle two-arg functions (not needed for now)
		for {
			tok3, ok3 := p.peek()
			if ok3 && tok3.kind == tokComma {
				p.advance()
				_, err = p.parseExpression() // consume second arg but ignore for now
				if err != nil {
					return 0, err
				}
				continue
			}
			break
		}

		tokClose, okClose := p.peek()
		if !okClose || tokClose.kind != tokRParen {
			return 0, fmt.Errorf("missing closing ')' for function %q", name)
		}
		p.advance()

		return evalFunc(name, arg)
	}

	// Number
	if tok.kind == tokNumber {
		p.advance()
		return strconv.ParseFloat(tok.value, 64)
	}

	return 0, fmt.Errorf("unexpected token: %q", tok.value)
}

func evalFunc(name string, arg float64) (float64, error) {
	switch name {
	case "sqrt":
		if arg < 0 {
			return 0, fmt.Errorf("sqrt of negative number")
		}
		return math.Sqrt(arg), nil
	case "sin":
		return math.Sin(arg), nil
	case "cos":
		return math.Cos(arg), nil
	case "tan":
		return math.Tan(arg), nil
	case "abs":
		return math.Abs(arg), nil
	case "round":
		return math.Round(arg), nil
	case "floor":
		return math.Floor(arg), nil
	case "ceil":
		return math.Ceil(arg), nil
	case "log":
		if arg <= 0 {
			return 0, fmt.Errorf("log of non-positive number")
		}
		return math.Log(arg), nil
	case "log10":
		if arg <= 0 {
			return 0, fmt.Errorf("log10 of non-positive number")
		}
		return math.Log10(arg), nil
	case "exp":
		return math.Exp(arg), nil
	default:
		return 0, fmt.Errorf("unknown function: %q", name)
	}
}
