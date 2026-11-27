package labelstructs

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// valuePathPattern matches a path rooted at values.yaml. Paths are kept simple
// on purpose: they must be usable directly as Go template fields, so dashes and
// other characters requiring the "index" function are rejected.
var valuePathPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// conditionKeyPattern detects the object form. A raw value starting with "!"
// is not valid YAML anyway, so the presence of a "condition:" line is a more
// reliable signal than a successful YAML probe.
var conditionKeyPattern = regexp.MustCompile(`(?m)^\s*condition\s*:`)

// Condition is a parsed katenary.v4/if label. It holds a boolean expression
// evaluated against values.yaml and the default values injected for the
// referenced flags.
type Condition struct {
	expr     expression
	defaults map[string]bool
}

// ConditionLabel is the katenary.yaml representation of the katenary.v4/if
// label. It accepts both the string form and the object form.
type ConditionLabel struct {
	Condition string          `yaml:"condition" json:"condition" jsonschema:"title=Condition,description=Boolean expression evaluated against values.yaml"`
	Defaults  map[string]bool `yaml:"defaults,omitempty" json:"defaults,omitempty" jsonschema:"title=Defaults,description=Default value of each flag referenced by the condition"`
}

// UnmarshalYAML accepts a plain expression or the object form.
func (c *ConditionLabel) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		return value.Decode(&c.Condition)
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("if must be a string or an object")
	}

	for i := 0; i < len(value.Content); i += 2 {
		switch value.Content[i].Value {
		case "condition", "defaults":
		default:
			return fmt.Errorf("unknown field %q in if", value.Content[i].Value)
		}
	}

	type plain ConditionLabel
	return value.Decode((*plain)(c))
}

// ConditionFrom parses the katenary.v4/if label value. Two forms are accepted:
//
//	proxy.enabled
//
// and the object form:
//
//	condition: proxy.enabled && !debug.enabled
//	defaults:
//	  proxy.enabled: true
//	  debug.enabled: false
func ConditionFrom(raw string) (*Condition, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("condition is empty")
	}

	// A plain expression such as "!proxy.enabled || tls.enabled" is not valid
	// YAML (the leading "!" is interpreted as a tag), so only treat the input
	// as an object when it actually decodes to a mapping. A "condition:" line is
	// also a strong signal, even when the value needs quoting.
	if conditionKeyPattern.MatchString(raw) {
		return conditionFromMapping(raw)
	}

	var probe any
	if err := yaml.Unmarshal([]byte(raw), &probe); err == nil {
		if _, isMapping := probe.(map[string]any); isMapping {
			return conditionFromMapping(raw)
		}
	}

	return conditionFromExpression(raw)
}

func conditionFromMapping(raw string) (*Condition, error) {
	var obj struct {
		Condition string          `yaml:"condition"`
		Defaults  map[string]bool `yaml:"defaults"`
	}

	decoder := yaml.NewDecoder(strings.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&obj); err != nil {
		return nil, fmt.Errorf("invalid condition object: %w (values starting with \"!\" must be quoted)", err)
	}
	if strings.TrimSpace(obj.Condition) == "" {
		return nil, fmt.Errorf(`missing "condition" key`)
	}

	condition, err := conditionFromExpression(obj.Condition)
	if err != nil {
		return nil, err
	}

	for path := range obj.Defaults {
		if !valuePathPattern.MatchString(path) {
			return nil, fmt.Errorf("invalid default path %q", path)
		}
	}
	condition.defaults = obj.Defaults
	return condition, nil
}

func conditionFromExpression(raw string) (*Condition, error) {
	tokens, err := lexCondition(raw)
	if err != nil {
		return nil, err
	}

	parser := &conditionParser{tokens: tokens}
	expr, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	if tok := parser.peek(); tok.kind != tokenEOF {
		return nil, fmt.Errorf("unexpected %s at position %d", tok.describe(), tok.pos+1)
	}
	return &Condition{expr: expr}, nil
}

// HelmExpr returns the Go template expression to use in an "if" action. It is
// built from Helm's and/or/not functions because Helm has no infix operators.
func (c *Condition) HelmExpr() string {
	return c.expr.helm()
}

// Paths returns the sorted, de-duplicated values.yaml paths used by the
// condition.
func (c *Condition) Paths() []string {
	set := map[string]struct{}{}
	c.expr.collectPaths(set)

	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// Default returns the default value to write in values.yaml for the given
// path. Flags without an explicit default are enabled by default.
func (c *Condition) Default(path string) bool {
	if value, ok := c.defaults[path]; ok {
		return value
	}
	return true
}

// UnusedDefaults returns the sorted default paths that the condition does not
// reference. They usually come from a typo.
func (c *Condition) UnusedDefaults() []string {
	used := map[string]struct{}{}
	for _, path := range c.Paths() {
		used[path] = struct{}{}
	}

	unused := make([]string, 0)
	for path := range c.defaults {
		if _, ok := used[path]; !ok {
			unused = append(unused, path)
		}
	}
	sort.Strings(unused)
	return unused
}

// expression is a node of the boolean expression tree.
type expression interface {
	helm() string
	collectPaths(set map[string]struct{})
}

type literalExpression bool

func (e literalExpression) helm() string {
	if e {
		return "true"
	}
	return "false"
}

func (literalExpression) collectPaths(map[string]struct{}) {}

type pathExpression string

func (e pathExpression) helm() string {
	return ".Values." + string(e)
}

func (e pathExpression) collectPaths(set map[string]struct{}) {
	set[string(e)] = struct{}{}
}

type notExpression struct {
	operand expression
}

func (e *notExpression) helm() string {
	return "not " + helmOperand(e.operand)
}

func (e *notExpression) collectPaths(set map[string]struct{}) {
	e.operand.collectPaths(set)
}

type binaryExpression struct {
	operator string
	operands []expression
}

func (e *binaryExpression) helm() string {
	parts := make([]string, len(e.operands))
	for i, operand := range e.operands {
		parts[i] = helmOperand(operand)
	}
	return e.operator + " " + strings.Join(parts, " ")
}

func (e *binaryExpression) collectPaths(set map[string]struct{}) {
	for _, operand := range e.operands {
		operand.collectPaths(set)
	}
}

// helmOperand parenthesizes nested expressions so that precedence is preserved
// with Helm's prefix functions.
func helmOperand(e expression) string {
	switch e.(type) {
	case *notExpression, *binaryExpression:
		return "(" + e.helm() + ")"
	default:
		return e.helm()
	}
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenAnd
	tokenOr
	tokenNot
	tokenLeftParen
	tokenRightParen
	tokenTrue
	tokenFalse
	tokenPath
)

type token struct {
	kind  tokenKind
	value string
	pos   int
}

func (t token) describe() string {
	switch t.kind {
	case tokenEOF:
		return "end of expression"
	case tokenAnd:
		return `"&&"`
	case tokenOr:
		return `"||"`
	case tokenNot:
		return `"!"`
	case tokenLeftParen:
		return `"("`
	case tokenRightParen:
		return `")"`
	case tokenTrue:
		return `"true"`
	case tokenFalse:
		return `"false"`
	case tokenPath:
		return fmt.Sprintf("value %q", t.value)
	default:
		return "token"
	}
}

func lexCondition(input string) ([]token, error) {
	tokens := make([]token, 0, 8)

	for i := 0; i < len(input); {
		switch c := input[i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			tokens = append(tokens, token{kind: tokenLeftParen, pos: i})
			i++
		case c == ')':
			tokens = append(tokens, token{kind: tokenRightParen, pos: i})
			i++
		case c == '!':
			tokens = append(tokens, token{kind: tokenNot, pos: i})
			i++
		case c == '&' || c == '|':
			if i+1 >= len(input) || input[i+1] != c {
				return nil, fmt.Errorf("unexpected %q at position %d (did you mean %q?)", c, i+1, string([]byte{c, c}))
			}
			kind := tokenAnd
			if c == '|' {
				kind = tokenOr
			}
			tokens = append(tokens, token{kind: kind, pos: i})
			i += 2
		case isWordStart(c):
			start := i
			for i < len(input) && isWordPart(input[i]) {
				i++
			}
			word := input[start:i]
			tokens = append(tokens, wordToken(word, start))
		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", c, i+1)
		}
	}

	tokens = append(tokens, token{kind: tokenEOF, pos: len(input)})
	return tokens, nil
}

func wordToken(word string, pos int) token {
	switch word {
	case "and":
		return token{kind: tokenAnd, value: word, pos: pos}
	case "or":
		return token{kind: tokenOr, value: word, pos: pos}
	case "not":
		return token{kind: tokenNot, value: word, pos: pos}
	case "true":
		return token{kind: tokenTrue, value: word, pos: pos}
	case "false":
		return token{kind: tokenFalse, value: word, pos: pos}
	}

	// A dotted name is never a keyword, so "not.ready" is a value path.
	return token{kind: tokenPath, value: word, pos: pos}
}

func isWordStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isWordPart(c byte) bool {
	return isWordStart(c) || (c >= '0' && c <= '9') || c == '.' || c == '-'
}

type conditionParser struct {
	tokens []token
	index  int
}

func (p *conditionParser) peek() token {
	return p.tokens[p.index]
}

func (p *conditionParser) next() token {
	tok := p.tokens[p.index]
	if tok.kind != tokenEOF {
		p.index++
	}
	return tok
}

func (p *conditionParser) parseOr() (expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	operands := []expression{left}
	for p.peek().kind == tokenOr {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		operands = append(operands, right)
	}
	if len(operands) == 1 {
		return left, nil
	}
	return &binaryExpression{operator: "or", operands: operands}, nil
}

func (p *conditionParser) parseAnd() (expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	operands := []expression{left}
	for p.peek().kind == tokenAnd {
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		operands = append(operands, right)
	}
	if len(operands) == 1 {
		return left, nil
	}
	return &binaryExpression{operator: "and", operands: operands}, nil
}

func (p *conditionParser) parseUnary() (expression, error) {
	tok := p.peek()

	switch tok.kind {
	case tokenNot:
		p.next()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &notExpression{operand: operand}, nil

	case tokenLeftParen:
		p.next()
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokenRightParen {
			return nil, fmt.Errorf("missing \")\" at position %d", p.peek().pos+1)
		}
		p.next()
		return expr, nil

	case tokenTrue:
		p.next()
		return literalExpression(true), nil

	case tokenFalse:
		p.next()
		return literalExpression(false), nil

	case tokenPath:
		p.next()
		if !valuePathPattern.MatchString(tok.value) {
			return nil, fmt.Errorf("invalid value path %q at position %d", tok.value, tok.pos+1)
		}
		return pathExpression(tok.value), nil

	case tokenEOF:
		return nil, fmt.Errorf("unexpected end of expression")

	default:
		return nil, fmt.Errorf("unexpected %s at position %d", tok.describe(), tok.pos+1)
	}
}
