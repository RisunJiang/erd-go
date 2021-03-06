package main

//go:generate peg erd.peg

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
)

const endSymbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleroot
	ruleEOT
	ruleexpression
	ruleempty_line
	rulecomment_line
	ruletitle_info
	ruletable_info
	ruletable_title
	ruletable_column
	rulecolumn_name
	rulerelation_info
	rulerelation_left
	rulecardinality_left
	rulerelation_right
	rulecardinality_right
	ruletitle_attribute
	ruletable_attribute
	rulecolumn_attribute
	rulerelation_attribute
	ruleattribute_key
	ruleattribute_value
	rulebare_value
	rulequoted_value
	ruleattribute_sep
	rulecomment_string
	rulews
	rulenewline
	rulenewline_or_eot
	rulespace
	rulestring
	rulestring_in_quote
	rulecardinality
	rulePegText
	ruleAction0
	ruleAction1
	ruleAction2
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8
	ruleAction9
	ruleAction10
	ruleAction11
	ruleAction12
	ruleAction13
	ruleAction14
	ruleAction15
	ruleAction16
)

var rul3s = [...]string{
	"Unknown",
	"root",
	"EOT",
	"expression",
	"empty_line",
	"comment_line",
	"title_info",
	"table_info",
	"table_title",
	"table_column",
	"column_name",
	"relation_info",
	"relation_left",
	"cardinality_left",
	"relation_right",
	"cardinality_right",
	"title_attribute",
	"table_attribute",
	"column_attribute",
	"relation_attribute",
	"attribute_key",
	"attribute_value",
	"bare_value",
	"quoted_value",
	"attribute_sep",
	"comment_string",
	"ws",
	"newline",
	"newline_or_eot",
	"space",
	"string",
	"string_in_quote",
	"cardinality",
	"PegText",
	"Action0",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",
	"Action10",
	"Action11",
	"Action12",
	"Action13",
	"Action14",
	"Action15",
	"Action16",
}

type token32 struct {
	pegRule
	begin, end uint32
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v", rul3s[t.pegRule], t.begin, t.end)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(w io.Writer, pretty bool, buffer string) {
	var print func(node *node32, depth int)
	print = func(node *node32, depth int) {
		for node != nil {
			for c := 0; c < depth; c++ {
				fmt.Fprintf(w, " ")
			}
			rule := rul3s[node.pegRule]
			quote := strconv.Quote(string(([]rune(buffer)[node.begin:node.end])))
			if !pretty {
				fmt.Fprintf(w, "%v %v\n", rule, quote)
			} else {
				fmt.Fprintf(w, "\x1B[34m%v\x1B[m %v\n", rule, quote)
			}
			if node.up != nil {
				print(node.up, depth+1)
			}
			node = node.next
		}
	}
	print(node, 0)
}

func (node *node32) Print(w io.Writer, buffer string) {
	node.print(w, false, buffer)
}

func (node *node32) PrettyPrint(w io.Writer, buffer string) {
	node.print(w, true, buffer)
}

type tokens32 struct {
	tree []token32
}

func (t *tokens32) Trim(length uint32) {
	t.tree = t.tree[:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) AST() *node32 {
	type element struct {
		node *node32
		down *element
	}
	tokens := t.Tokens()
	var stack *element
	for _, token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	if stack != nil {
		return stack.node
	}
	return nil
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	t.AST().Print(os.Stdout, buffer)
}

func (t *tokens32) WriteSyntaxTree(w io.Writer, buffer string) {
	t.AST().Print(w, buffer)
}

func (t *tokens32) PrettyPrintSyntaxTree(buffer string) {
	t.AST().PrettyPrint(os.Stdout, buffer)
}

func (t *tokens32) Add(rule pegRule, begin, end, index uint32) {
	if tree := t.tree; int(index) >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	t.tree[index] = token32{
		pegRule: rule,
		begin:   begin,
		end:     end,
	}
}

func (t *tokens32) Tokens() []token32 {
	return t.tree
}

type Parser struct {
	Erd

	Buffer string
	buffer []rune
	rules  [51]func() bool
	parse  func(rule ...int) error
	reset  func()
	Pretty bool
	tokens32
}

func (p *Parser) Parse(rule ...int) error {
	return p.parse(rule...)
}

func (p *Parser) Reset() {
	p.reset()
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *Parser
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *Parser) PrintSyntaxTree() {
	if p.Pretty {
		p.tokens32.PrettyPrintSyntaxTree(p.Buffer)
	} else {
		p.tokens32.PrintSyntaxTree(p.Buffer)
	}
}

func (p *Parser) WriteSyntaxTree(w io.Writer) {
	p.tokens32.WriteSyntaxTree(w, p.Buffer)
}

func (p *Parser) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for _, token := range p.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
			text = string(_buffer[begin:end])

		case ruleAction0:
			p.Err(begin, buffer)
		case ruleAction1:
			p.Err(begin, buffer)
		case ruleAction2:
			p.ClearTableAndColumn()
		case ruleAction3:
			p.AddTable(text)
		case ruleAction4:
			p.AddColumn(text)
		case ruleAction5:
			p.AddRelation()
		case ruleAction6:
			p.SetRelationLeft(text)
		case ruleAction7:
			p.SetCardinalityLeft(text)
		case ruleAction8:
			p.SetRelationRight(text)
		case ruleAction9:
			p.SetCardinalityRight(text)
		case ruleAction10:
			p.AddTitleKeyValue()
		case ruleAction11:
			p.AddTableKeyValue()
		case ruleAction12:
			p.AddColumnKeyValue()
		case ruleAction13:
			p.AddRelationKeyValue()
		case ruleAction14:
			p.SetKey(text)
		case ruleAction15:
			p.SetValue(text)
		case ruleAction16:
			p.SetValue(text)

		}
	}
	_, _, _, _, _ = buffer, _buffer, text, begin, end
}

func (p *Parser) Init() {
	var (
		max                  token32
		position, tokenIndex uint32
		buffer               []rune
	)
	p.reset = func() {
		max = token32{}
		position, tokenIndex = 0, 0

		p.buffer = []rune(p.Buffer)
		if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
			p.buffer = append(p.buffer, endSymbol)
		}
		buffer = p.buffer
	}
	p.reset()

	_rules := p.rules
	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	p.parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.Trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	add := func(rule pegRule, begin uint32) {
		tree.Add(rule, begin, position, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position}
		}
	}

	matchDot := func() bool {
		if buffer[position] != endSymbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 root <- <((expression EOT) / (expression <.+> Action0 EOT) / (<.+> Action1 EOT))> */
		func() bool {
			position0, tokenIndex0 := position, tokenIndex
			{
				position1 := position
				{
					position2, tokenIndex2 := position, tokenIndex
					if !_rules[ruleexpression]() {
						goto l3
					}
					if !_rules[ruleEOT]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex = position2, tokenIndex2
					if !_rules[ruleexpression]() {
						goto l4
					}
					{
						position5 := position
						if !matchDot() {
							goto l4
						}
					l6:
						{
							position7, tokenIndex7 := position, tokenIndex
							if !matchDot() {
								goto l7
							}
							goto l6
						l7:
							position, tokenIndex = position7, tokenIndex7
						}
						add(rulePegText, position5)
					}
					if !_rules[ruleAction0]() {
						goto l4
					}
					if !_rules[ruleEOT]() {
						goto l4
					}
					goto l2
				l4:
					position, tokenIndex = position2, tokenIndex2
					{
						position8 := position
						if !matchDot() {
							goto l0
						}
					l9:
						{
							position10, tokenIndex10 := position, tokenIndex
							if !matchDot() {
								goto l10
							}
							goto l9
						l10:
							position, tokenIndex = position10, tokenIndex10
						}
						add(rulePegText, position8)
					}
					if !_rules[ruleAction1]() {
						goto l0
					}
					if !_rules[ruleEOT]() {
						goto l0
					}
				}
			l2:
				add(ruleroot, position1)
			}
			return true
		l0:
			position, tokenIndex = position0, tokenIndex0
			return false
		},
		/* 1 EOT <- <!.> */
		func() bool {
			position11, tokenIndex11 := position, tokenIndex
			{
				position12 := position
				{
					position13, tokenIndex13 := position, tokenIndex
					if !matchDot() {
						goto l13
					}
					goto l11
				l13:
					position, tokenIndex = position13, tokenIndex13
				}
				add(ruleEOT, position12)
			}
			return true
		l11:
			position, tokenIndex = position11, tokenIndex11
			return false
		},
		/* 2 expression <- <(title_info / relation_info / table_info / comment_line / empty_line)*> */
		func() bool {
			{
				position15 := position
			l16:
				{
					position17, tokenIndex17 := position, tokenIndex
					{
						position18, tokenIndex18 := position, tokenIndex
						if !_rules[ruletitle_info]() {
							goto l19
						}
						goto l18
					l19:
						position, tokenIndex = position18, tokenIndex18
						if !_rules[rulerelation_info]() {
							goto l20
						}
						goto l18
					l20:
						position, tokenIndex = position18, tokenIndex18
						if !_rules[ruletable_info]() {
							goto l21
						}
						goto l18
					l21:
						position, tokenIndex = position18, tokenIndex18
						if !_rules[rulecomment_line]() {
							goto l22
						}
						goto l18
					l22:
						position, tokenIndex = position18, tokenIndex18
						if !_rules[ruleempty_line]() {
							goto l17
						}
					}
				l18:
					goto l16
				l17:
					position, tokenIndex = position17, tokenIndex17
				}
				add(ruleexpression, position15)
			}
			return true
		},
		/* 3 empty_line <- <(ws Action2)> */
		func() bool {
			position23, tokenIndex23 := position, tokenIndex
			{
				position24 := position
				if !_rules[rulews]() {
					goto l23
				}
				if !_rules[ruleAction2]() {
					goto l23
				}
				add(ruleempty_line, position24)
			}
			return true
		l23:
			position, tokenIndex = position23, tokenIndex23
			return false
		},
		/* 4 comment_line <- <(space* '#' comment_string newline)> */
		func() bool {
			position25, tokenIndex25 := position, tokenIndex
			{
				position26 := position
			l27:
				{
					position28, tokenIndex28 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l28
					}
					goto l27
				l28:
					position, tokenIndex = position28, tokenIndex28
				}
				if buffer[position] != rune('#') {
					goto l25
				}
				position++
				if !_rules[rulecomment_string]() {
					goto l25
				}
				if !_rules[rulenewline]() {
					goto l25
				}
				add(rulecomment_line, position26)
			}
			return true
		l25:
			position, tokenIndex = position25, tokenIndex25
			return false
		},
		/* 5 title_info <- <('t' 'i' 't' 'l' 'e' ws* '{' ws* (title_attribute ws* attribute_sep? ws*)* ws* '}' newline)> */
		func() bool {
			position29, tokenIndex29 := position, tokenIndex
			{
				position30 := position
				if buffer[position] != rune('t') {
					goto l29
				}
				position++
				if buffer[position] != rune('i') {
					goto l29
				}
				position++
				if buffer[position] != rune('t') {
					goto l29
				}
				position++
				if buffer[position] != rune('l') {
					goto l29
				}
				position++
				if buffer[position] != rune('e') {
					goto l29
				}
				position++
			l31:
				{
					position32, tokenIndex32 := position, tokenIndex
					if !_rules[rulews]() {
						goto l32
					}
					goto l31
				l32:
					position, tokenIndex = position32, tokenIndex32
				}
				if buffer[position] != rune('{') {
					goto l29
				}
				position++
			l33:
				{
					position34, tokenIndex34 := position, tokenIndex
					if !_rules[rulews]() {
						goto l34
					}
					goto l33
				l34:
					position, tokenIndex = position34, tokenIndex34
				}
			l35:
				{
					position36, tokenIndex36 := position, tokenIndex
					if !_rules[ruletitle_attribute]() {
						goto l36
					}
				l37:
					{
						position38, tokenIndex38 := position, tokenIndex
						if !_rules[rulews]() {
							goto l38
						}
						goto l37
					l38:
						position, tokenIndex = position38, tokenIndex38
					}
					{
						position39, tokenIndex39 := position, tokenIndex
						if !_rules[ruleattribute_sep]() {
							goto l39
						}
						goto l40
					l39:
						position, tokenIndex = position39, tokenIndex39
					}
				l40:
				l41:
					{
						position42, tokenIndex42 := position, tokenIndex
						if !_rules[rulews]() {
							goto l42
						}
						goto l41
					l42:
						position, tokenIndex = position42, tokenIndex42
					}
					goto l35
				l36:
					position, tokenIndex = position36, tokenIndex36
				}
			l43:
				{
					position44, tokenIndex44 := position, tokenIndex
					if !_rules[rulews]() {
						goto l44
					}
					goto l43
				l44:
					position, tokenIndex = position44, tokenIndex44
				}
				if buffer[position] != rune('}') {
					goto l29
				}
				position++
				if !_rules[rulenewline]() {
					goto l29
				}
				add(ruletitle_info, position30)
			}
			return true
		l29:
			position, tokenIndex = position29, tokenIndex29
			return false
		},
		/* 6 table_info <- <('[' table_title ']' (space* '{' ws* (table_attribute ws* attribute_sep?)* ws* '}' space*)? newline_or_eot (table_column / empty_line)*)> */
		func() bool {
			position45, tokenIndex45 := position, tokenIndex
			{
				position46 := position
				if buffer[position] != rune('[') {
					goto l45
				}
				position++
				if !_rules[ruletable_title]() {
					goto l45
				}
				if buffer[position] != rune(']') {
					goto l45
				}
				position++
				{
					position47, tokenIndex47 := position, tokenIndex
				l49:
					{
						position50, tokenIndex50 := position, tokenIndex
						if !_rules[rulespace]() {
							goto l50
						}
						goto l49
					l50:
						position, tokenIndex = position50, tokenIndex50
					}
					if buffer[position] != rune('{') {
						goto l47
					}
					position++
				l51:
					{
						position52, tokenIndex52 := position, tokenIndex
						if !_rules[rulews]() {
							goto l52
						}
						goto l51
					l52:
						position, tokenIndex = position52, tokenIndex52
					}
				l53:
					{
						position54, tokenIndex54 := position, tokenIndex
						if !_rules[ruletable_attribute]() {
							goto l54
						}
					l55:
						{
							position56, tokenIndex56 := position, tokenIndex
							if !_rules[rulews]() {
								goto l56
							}
							goto l55
						l56:
							position, tokenIndex = position56, tokenIndex56
						}
						{
							position57, tokenIndex57 := position, tokenIndex
							if !_rules[ruleattribute_sep]() {
								goto l57
							}
							goto l58
						l57:
							position, tokenIndex = position57, tokenIndex57
						}
					l58:
						goto l53
					l54:
						position, tokenIndex = position54, tokenIndex54
					}
				l59:
					{
						position60, tokenIndex60 := position, tokenIndex
						if !_rules[rulews]() {
							goto l60
						}
						goto l59
					l60:
						position, tokenIndex = position60, tokenIndex60
					}
					if buffer[position] != rune('}') {
						goto l47
					}
					position++
				l61:
					{
						position62, tokenIndex62 := position, tokenIndex
						if !_rules[rulespace]() {
							goto l62
						}
						goto l61
					l62:
						position, tokenIndex = position62, tokenIndex62
					}
					goto l48
				l47:
					position, tokenIndex = position47, tokenIndex47
				}
			l48:
				if !_rules[rulenewline_or_eot]() {
					goto l45
				}
			l63:
				{
					position64, tokenIndex64 := position, tokenIndex
					{
						position65, tokenIndex65 := position, tokenIndex
						if !_rules[ruletable_column]() {
							goto l66
						}
						goto l65
					l66:
						position, tokenIndex = position65, tokenIndex65
						if !_rules[ruleempty_line]() {
							goto l64
						}
					}
				l65:
					goto l63
				l64:
					position, tokenIndex = position64, tokenIndex64
				}
				add(ruletable_info, position46)
			}
			return true
		l45:
			position, tokenIndex = position45, tokenIndex45
			return false
		},
		/* 7 table_title <- <(<string> Action3)> */
		func() bool {
			position67, tokenIndex67 := position, tokenIndex
			{
				position68 := position
				{
					position69 := position
					if !_rules[rulestring]() {
						goto l67
					}
					add(rulePegText, position69)
				}
				if !_rules[ruleAction3]() {
					goto l67
				}
				add(ruletable_title, position68)
			}
			return true
		l67:
			position, tokenIndex = position67, tokenIndex67
			return false
		},
		/* 8 table_column <- <(space* column_name (space* '{' ws* (column_attribute ws* attribute_sep?)* ws* '}' space*)? newline_or_eot)> */
		func() bool {
			position70, tokenIndex70 := position, tokenIndex
			{
				position71 := position
			l72:
				{
					position73, tokenIndex73 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l73
					}
					goto l72
				l73:
					position, tokenIndex = position73, tokenIndex73
				}
				if !_rules[rulecolumn_name]() {
					goto l70
				}
				{
					position74, tokenIndex74 := position, tokenIndex
				l76:
					{
						position77, tokenIndex77 := position, tokenIndex
						if !_rules[rulespace]() {
							goto l77
						}
						goto l76
					l77:
						position, tokenIndex = position77, tokenIndex77
					}
					if buffer[position] != rune('{') {
						goto l74
					}
					position++
				l78:
					{
						position79, tokenIndex79 := position, tokenIndex
						if !_rules[rulews]() {
							goto l79
						}
						goto l78
					l79:
						position, tokenIndex = position79, tokenIndex79
					}
				l80:
					{
						position81, tokenIndex81 := position, tokenIndex
						if !_rules[rulecolumn_attribute]() {
							goto l81
						}
					l82:
						{
							position83, tokenIndex83 := position, tokenIndex
							if !_rules[rulews]() {
								goto l83
							}
							goto l82
						l83:
							position, tokenIndex = position83, tokenIndex83
						}
						{
							position84, tokenIndex84 := position, tokenIndex
							if !_rules[ruleattribute_sep]() {
								goto l84
							}
							goto l85
						l84:
							position, tokenIndex = position84, tokenIndex84
						}
					l85:
						goto l80
					l81:
						position, tokenIndex = position81, tokenIndex81
					}
				l86:
					{
						position87, tokenIndex87 := position, tokenIndex
						if !_rules[rulews]() {
							goto l87
						}
						goto l86
					l87:
						position, tokenIndex = position87, tokenIndex87
					}
					if buffer[position] != rune('}') {
						goto l74
					}
					position++
				l88:
					{
						position89, tokenIndex89 := position, tokenIndex
						if !_rules[rulespace]() {
							goto l89
						}
						goto l88
					l89:
						position, tokenIndex = position89, tokenIndex89
					}
					goto l75
				l74:
					position, tokenIndex = position74, tokenIndex74
				}
			l75:
				if !_rules[rulenewline_or_eot]() {
					goto l70
				}
				add(ruletable_column, position71)
			}
			return true
		l70:
			position, tokenIndex = position70, tokenIndex70
			return false
		},
		/* 9 column_name <- <(<string> Action4)> */
		func() bool {
			position90, tokenIndex90 := position, tokenIndex
			{
				position91 := position
				{
					position92 := position
					if !_rules[rulestring]() {
						goto l90
					}
					add(rulePegText, position92)
				}
				if !_rules[ruleAction4]() {
					goto l90
				}
				add(rulecolumn_name, position91)
			}
			return true
		l90:
			position, tokenIndex = position90, tokenIndex90
			return false
		},
		/* 10 relation_info <- <(space* relation_left space* cardinality_left ('-' '-') cardinality_right space* relation_right (ws* '{' ws* (relation_attribute ws* attribute_sep? ws*)* ws* '}')? newline_or_eot Action5)> */
		func() bool {
			position93, tokenIndex93 := position, tokenIndex
			{
				position94 := position
			l95:
				{
					position96, tokenIndex96 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l96
					}
					goto l95
				l96:
					position, tokenIndex = position96, tokenIndex96
				}
				if !_rules[rulerelation_left]() {
					goto l93
				}
			l97:
				{
					position98, tokenIndex98 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l98
					}
					goto l97
				l98:
					position, tokenIndex = position98, tokenIndex98
				}
				if !_rules[rulecardinality_left]() {
					goto l93
				}
				if buffer[position] != rune('-') {
					goto l93
				}
				position++
				if buffer[position] != rune('-') {
					goto l93
				}
				position++
				if !_rules[rulecardinality_right]() {
					goto l93
				}
			l99:
				{
					position100, tokenIndex100 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l100
					}
					goto l99
				l100:
					position, tokenIndex = position100, tokenIndex100
				}
				if !_rules[rulerelation_right]() {
					goto l93
				}
				{
					position101, tokenIndex101 := position, tokenIndex
				l103:
					{
						position104, tokenIndex104 := position, tokenIndex
						if !_rules[rulews]() {
							goto l104
						}
						goto l103
					l104:
						position, tokenIndex = position104, tokenIndex104
					}
					if buffer[position] != rune('{') {
						goto l101
					}
					position++
				l105:
					{
						position106, tokenIndex106 := position, tokenIndex
						if !_rules[rulews]() {
							goto l106
						}
						goto l105
					l106:
						position, tokenIndex = position106, tokenIndex106
					}
				l107:
					{
						position108, tokenIndex108 := position, tokenIndex
						if !_rules[rulerelation_attribute]() {
							goto l108
						}
					l109:
						{
							position110, tokenIndex110 := position, tokenIndex
							if !_rules[rulews]() {
								goto l110
							}
							goto l109
						l110:
							position, tokenIndex = position110, tokenIndex110
						}
						{
							position111, tokenIndex111 := position, tokenIndex
							if !_rules[ruleattribute_sep]() {
								goto l111
							}
							goto l112
						l111:
							position, tokenIndex = position111, tokenIndex111
						}
					l112:
					l113:
						{
							position114, tokenIndex114 := position, tokenIndex
							if !_rules[rulews]() {
								goto l114
							}
							goto l113
						l114:
							position, tokenIndex = position114, tokenIndex114
						}
						goto l107
					l108:
						position, tokenIndex = position108, tokenIndex108
					}
				l115:
					{
						position116, tokenIndex116 := position, tokenIndex
						if !_rules[rulews]() {
							goto l116
						}
						goto l115
					l116:
						position, tokenIndex = position116, tokenIndex116
					}
					if buffer[position] != rune('}') {
						goto l101
					}
					position++
					goto l102
				l101:
					position, tokenIndex = position101, tokenIndex101
				}
			l102:
				if !_rules[rulenewline_or_eot]() {
					goto l93
				}
				if !_rules[ruleAction5]() {
					goto l93
				}
				add(rulerelation_info, position94)
			}
			return true
		l93:
			position, tokenIndex = position93, tokenIndex93
			return false
		},
		/* 11 relation_left <- <(<string> Action6)> */
		func() bool {
			position117, tokenIndex117 := position, tokenIndex
			{
				position118 := position
				{
					position119 := position
					if !_rules[rulestring]() {
						goto l117
					}
					add(rulePegText, position119)
				}
				if !_rules[ruleAction6]() {
					goto l117
				}
				add(rulerelation_left, position118)
			}
			return true
		l117:
			position, tokenIndex = position117, tokenIndex117
			return false
		},
		/* 12 cardinality_left <- <(<cardinality> Action7)> */
		func() bool {
			position120, tokenIndex120 := position, tokenIndex
			{
				position121 := position
				{
					position122 := position
					if !_rules[rulecardinality]() {
						goto l120
					}
					add(rulePegText, position122)
				}
				if !_rules[ruleAction7]() {
					goto l120
				}
				add(rulecardinality_left, position121)
			}
			return true
		l120:
			position, tokenIndex = position120, tokenIndex120
			return false
		},
		/* 13 relation_right <- <(<string> Action8)> */
		func() bool {
			position123, tokenIndex123 := position, tokenIndex
			{
				position124 := position
				{
					position125 := position
					if !_rules[rulestring]() {
						goto l123
					}
					add(rulePegText, position125)
				}
				if !_rules[ruleAction8]() {
					goto l123
				}
				add(rulerelation_right, position124)
			}
			return true
		l123:
			position, tokenIndex = position123, tokenIndex123
			return false
		},
		/* 14 cardinality_right <- <(<cardinality> Action9)> */
		func() bool {
			position126, tokenIndex126 := position, tokenIndex
			{
				position127 := position
				{
					position128 := position
					if !_rules[rulecardinality]() {
						goto l126
					}
					add(rulePegText, position128)
				}
				if !_rules[ruleAction9]() {
					goto l126
				}
				add(rulecardinality_right, position127)
			}
			return true
		l126:
			position, tokenIndex = position126, tokenIndex126
			return false
		},
		/* 15 title_attribute <- <(attribute_key space* ':' space* attribute_value Action10)> */
		func() bool {
			position129, tokenIndex129 := position, tokenIndex
			{
				position130 := position
				if !_rules[ruleattribute_key]() {
					goto l129
				}
			l131:
				{
					position132, tokenIndex132 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l132
					}
					goto l131
				l132:
					position, tokenIndex = position132, tokenIndex132
				}
				if buffer[position] != rune(':') {
					goto l129
				}
				position++
			l133:
				{
					position134, tokenIndex134 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l134
					}
					goto l133
				l134:
					position, tokenIndex = position134, tokenIndex134
				}
				if !_rules[ruleattribute_value]() {
					goto l129
				}
				if !_rules[ruleAction10]() {
					goto l129
				}
				add(ruletitle_attribute, position130)
			}
			return true
		l129:
			position, tokenIndex = position129, tokenIndex129
			return false
		},
		/* 16 table_attribute <- <(attribute_key space* ':' space* attribute_value Action11)> */
		func() bool {
			position135, tokenIndex135 := position, tokenIndex
			{
				position136 := position
				if !_rules[ruleattribute_key]() {
					goto l135
				}
			l137:
				{
					position138, tokenIndex138 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l138
					}
					goto l137
				l138:
					position, tokenIndex = position138, tokenIndex138
				}
				if buffer[position] != rune(':') {
					goto l135
				}
				position++
			l139:
				{
					position140, tokenIndex140 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l140
					}
					goto l139
				l140:
					position, tokenIndex = position140, tokenIndex140
				}
				if !_rules[ruleattribute_value]() {
					goto l135
				}
				if !_rules[ruleAction11]() {
					goto l135
				}
				add(ruletable_attribute, position136)
			}
			return true
		l135:
			position, tokenIndex = position135, tokenIndex135
			return false
		},
		/* 17 column_attribute <- <(attribute_key space* ':' space* attribute_value Action12)> */
		func() bool {
			position141, tokenIndex141 := position, tokenIndex
			{
				position142 := position
				if !_rules[ruleattribute_key]() {
					goto l141
				}
			l143:
				{
					position144, tokenIndex144 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l144
					}
					goto l143
				l144:
					position, tokenIndex = position144, tokenIndex144
				}
				if buffer[position] != rune(':') {
					goto l141
				}
				position++
			l145:
				{
					position146, tokenIndex146 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l146
					}
					goto l145
				l146:
					position, tokenIndex = position146, tokenIndex146
				}
				if !_rules[ruleattribute_value]() {
					goto l141
				}
				if !_rules[ruleAction12]() {
					goto l141
				}
				add(rulecolumn_attribute, position142)
			}
			return true
		l141:
			position, tokenIndex = position141, tokenIndex141
			return false
		},
		/* 18 relation_attribute <- <(attribute_key space* ':' space* attribute_value Action13)> */
		func() bool {
			position147, tokenIndex147 := position, tokenIndex
			{
				position148 := position
				if !_rules[ruleattribute_key]() {
					goto l147
				}
			l149:
				{
					position150, tokenIndex150 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l150
					}
					goto l149
				l150:
					position, tokenIndex = position150, tokenIndex150
				}
				if buffer[position] != rune(':') {
					goto l147
				}
				position++
			l151:
				{
					position152, tokenIndex152 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l152
					}
					goto l151
				l152:
					position, tokenIndex = position152, tokenIndex152
				}
				if !_rules[ruleattribute_value]() {
					goto l147
				}
				if !_rules[ruleAction13]() {
					goto l147
				}
				add(rulerelation_attribute, position148)
			}
			return true
		l147:
			position, tokenIndex = position147, tokenIndex147
			return false
		},
		/* 19 attribute_key <- <(<string> Action14)> */
		func() bool {
			position153, tokenIndex153 := position, tokenIndex
			{
				position154 := position
				{
					position155 := position
					if !_rules[rulestring]() {
						goto l153
					}
					add(rulePegText, position155)
				}
				if !_rules[ruleAction14]() {
					goto l153
				}
				add(ruleattribute_key, position154)
			}
			return true
		l153:
			position, tokenIndex = position153, tokenIndex153
			return false
		},
		/* 20 attribute_value <- <(bare_value / quoted_value)> */
		func() bool {
			position156, tokenIndex156 := position, tokenIndex
			{
				position157 := position
				{
					position158, tokenIndex158 := position, tokenIndex
					if !_rules[rulebare_value]() {
						goto l159
					}
					goto l158
				l159:
					position, tokenIndex = position158, tokenIndex158
					if !_rules[rulequoted_value]() {
						goto l156
					}
				}
			l158:
				add(ruleattribute_value, position157)
			}
			return true
		l156:
			position, tokenIndex = position156, tokenIndex156
			return false
		},
		/* 21 bare_value <- <(<string> Action15)> */
		func() bool {
			position160, tokenIndex160 := position, tokenIndex
			{
				position161 := position
				{
					position162 := position
					if !_rules[rulestring]() {
						goto l160
					}
					add(rulePegText, position162)
				}
				if !_rules[ruleAction15]() {
					goto l160
				}
				add(rulebare_value, position161)
			}
			return true
		l160:
			position, tokenIndex = position160, tokenIndex160
			return false
		},
		/* 22 quoted_value <- <(<('"' string_in_quote '"')> Action16)> */
		func() bool {
			position163, tokenIndex163 := position, tokenIndex
			{
				position164 := position
				{
					position165 := position
					if buffer[position] != rune('"') {
						goto l163
					}
					position++
					if !_rules[rulestring_in_quote]() {
						goto l163
					}
					if buffer[position] != rune('"') {
						goto l163
					}
					position++
					add(rulePegText, position165)
				}
				if !_rules[ruleAction16]() {
					goto l163
				}
				add(rulequoted_value, position164)
			}
			return true
		l163:
			position, tokenIndex = position163, tokenIndex163
			return false
		},
		/* 23 attribute_sep <- <(space* ',' space*)> */
		func() bool {
			position166, tokenIndex166 := position, tokenIndex
			{
				position167 := position
			l168:
				{
					position169, tokenIndex169 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l169
					}
					goto l168
				l169:
					position, tokenIndex = position169, tokenIndex169
				}
				if buffer[position] != rune(',') {
					goto l166
				}
				position++
			l170:
				{
					position171, tokenIndex171 := position, tokenIndex
					if !_rules[rulespace]() {
						goto l171
					}
					goto l170
				l171:
					position, tokenIndex = position171, tokenIndex171
				}
				add(ruleattribute_sep, position167)
			}
			return true
		l166:
			position, tokenIndex = position166, tokenIndex166
			return false
		},
		/* 24 comment_string <- <(!('\r' / '\n') .)*> */
		func() bool {
			{
				position173 := position
			l174:
				{
					position175, tokenIndex175 := position, tokenIndex
					{
						position176, tokenIndex176 := position, tokenIndex
						{
							position177, tokenIndex177 := position, tokenIndex
							if buffer[position] != rune('\r') {
								goto l178
							}
							position++
							goto l177
						l178:
							position, tokenIndex = position177, tokenIndex177
							if buffer[position] != rune('\n') {
								goto l176
							}
							position++
						}
					l177:
						goto l175
					l176:
						position, tokenIndex = position176, tokenIndex176
					}
					if !matchDot() {
						goto l175
					}
					goto l174
				l175:
					position, tokenIndex = position175, tokenIndex175
				}
				add(rulecomment_string, position173)
			}
			return true
		},
		/* 25 ws <- <(' ' / '\t' / '\r' / '\n')+> */
		func() bool {
			position179, tokenIndex179 := position, tokenIndex
			{
				position180 := position
				{
					position183, tokenIndex183 := position, tokenIndex
					if buffer[position] != rune(' ') {
						goto l184
					}
					position++
					goto l183
				l184:
					position, tokenIndex = position183, tokenIndex183
					if buffer[position] != rune('\t') {
						goto l185
					}
					position++
					goto l183
				l185:
					position, tokenIndex = position183, tokenIndex183
					if buffer[position] != rune('\r') {
						goto l186
					}
					position++
					goto l183
				l186:
					position, tokenIndex = position183, tokenIndex183
					if buffer[position] != rune('\n') {
						goto l179
					}
					position++
				}
			l183:
			l181:
				{
					position182, tokenIndex182 := position, tokenIndex
					{
						position187, tokenIndex187 := position, tokenIndex
						if buffer[position] != rune(' ') {
							goto l188
						}
						position++
						goto l187
					l188:
						position, tokenIndex = position187, tokenIndex187
						if buffer[position] != rune('\t') {
							goto l189
						}
						position++
						goto l187
					l189:
						position, tokenIndex = position187, tokenIndex187
						if buffer[position] != rune('\r') {
							goto l190
						}
						position++
						goto l187
					l190:
						position, tokenIndex = position187, tokenIndex187
						if buffer[position] != rune('\n') {
							goto l182
						}
						position++
					}
				l187:
					goto l181
				l182:
					position, tokenIndex = position182, tokenIndex182
				}
				add(rulews, position180)
			}
			return true
		l179:
			position, tokenIndex = position179, tokenIndex179
			return false
		},
		/* 26 newline <- <(('\r' '\n') / '\n' / '\r')> */
		func() bool {
			position191, tokenIndex191 := position, tokenIndex
			{
				position192 := position
				{
					position193, tokenIndex193 := position, tokenIndex
					if buffer[position] != rune('\r') {
						goto l194
					}
					position++
					if buffer[position] != rune('\n') {
						goto l194
					}
					position++
					goto l193
				l194:
					position, tokenIndex = position193, tokenIndex193
					if buffer[position] != rune('\n') {
						goto l195
					}
					position++
					goto l193
				l195:
					position, tokenIndex = position193, tokenIndex193
					if buffer[position] != rune('\r') {
						goto l191
					}
					position++
				}
			l193:
				add(rulenewline, position192)
			}
			return true
		l191:
			position, tokenIndex = position191, tokenIndex191
			return false
		},
		/* 27 newline_or_eot <- <(newline / EOT)> */
		func() bool {
			position196, tokenIndex196 := position, tokenIndex
			{
				position197 := position
				{
					position198, tokenIndex198 := position, tokenIndex
					if !_rules[rulenewline]() {
						goto l199
					}
					goto l198
				l199:
					position, tokenIndex = position198, tokenIndex198
					if !_rules[ruleEOT]() {
						goto l196
					}
				}
			l198:
				add(rulenewline_or_eot, position197)
			}
			return true
		l196:
			position, tokenIndex = position196, tokenIndex196
			return false
		},
		/* 28 space <- <(' ' / '\t')+> */
		func() bool {
			position200, tokenIndex200 := position, tokenIndex
			{
				position201 := position
				{
					position204, tokenIndex204 := position, tokenIndex
					if buffer[position] != rune(' ') {
						goto l205
					}
					position++
					goto l204
				l205:
					position, tokenIndex = position204, tokenIndex204
					if buffer[position] != rune('\t') {
						goto l200
					}
					position++
				}
			l204:
			l202:
				{
					position203, tokenIndex203 := position, tokenIndex
					{
						position206, tokenIndex206 := position, tokenIndex
						if buffer[position] != rune(' ') {
							goto l207
						}
						position++
						goto l206
					l207:
						position, tokenIndex = position206, tokenIndex206
						if buffer[position] != rune('\t') {
							goto l203
						}
						position++
					}
				l206:
					goto l202
				l203:
					position, tokenIndex = position203, tokenIndex203
				}
				add(rulespace, position201)
			}
			return true
		l200:
			position, tokenIndex = position200, tokenIndex200
			return false
		},
		/* 29 string <- <(!('"' / '\t' / '\r' / '\n' / '/' / ':' / ',' / '[' / ']' / '{' / '}' / ' ') .)+> */
		func() bool {
			position208, tokenIndex208 := position, tokenIndex
			{
				position209 := position
				{
					position212, tokenIndex212 := position, tokenIndex
					{
						position213, tokenIndex213 := position, tokenIndex
						if buffer[position] != rune('"') {
							goto l214
						}
						position++
						goto l213
					l214:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune('\t') {
							goto l215
						}
						position++
						goto l213
					l215:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune('\r') {
							goto l216
						}
						position++
						goto l213
					l216:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune('\n') {
							goto l217
						}
						position++
						goto l213
					l217:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune('/') {
							goto l218
						}
						position++
						goto l213
					l218:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune(':') {
							goto l219
						}
						position++
						goto l213
					l219:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune(',') {
							goto l220
						}
						position++
						goto l213
					l220:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune('[') {
							goto l221
						}
						position++
						goto l213
					l221:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune(']') {
							goto l222
						}
						position++
						goto l213
					l222:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune('{') {
							goto l223
						}
						position++
						goto l213
					l223:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune('}') {
							goto l224
						}
						position++
						goto l213
					l224:
						position, tokenIndex = position213, tokenIndex213
						if buffer[position] != rune(' ') {
							goto l212
						}
						position++
					}
				l213:
					goto l208
				l212:
					position, tokenIndex = position212, tokenIndex212
				}
				if !matchDot() {
					goto l208
				}
			l210:
				{
					position211, tokenIndex211 := position, tokenIndex
					{
						position225, tokenIndex225 := position, tokenIndex
						{
							position226, tokenIndex226 := position, tokenIndex
							if buffer[position] != rune('"') {
								goto l227
							}
							position++
							goto l226
						l227:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune('\t') {
								goto l228
							}
							position++
							goto l226
						l228:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune('\r') {
								goto l229
							}
							position++
							goto l226
						l229:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune('\n') {
								goto l230
							}
							position++
							goto l226
						l230:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune('/') {
								goto l231
							}
							position++
							goto l226
						l231:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune(':') {
								goto l232
							}
							position++
							goto l226
						l232:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune(',') {
								goto l233
							}
							position++
							goto l226
						l233:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune('[') {
								goto l234
							}
							position++
							goto l226
						l234:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune(']') {
								goto l235
							}
							position++
							goto l226
						l235:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune('{') {
								goto l236
							}
							position++
							goto l226
						l236:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune('}') {
								goto l237
							}
							position++
							goto l226
						l237:
							position, tokenIndex = position226, tokenIndex226
							if buffer[position] != rune(' ') {
								goto l225
							}
							position++
						}
					l226:
						goto l211
					l225:
						position, tokenIndex = position225, tokenIndex225
					}
					if !matchDot() {
						goto l211
					}
					goto l210
				l211:
					position, tokenIndex = position211, tokenIndex211
				}
				add(rulestring, position209)
			}
			return true
		l208:
			position, tokenIndex = position208, tokenIndex208
			return false
		},
		/* 30 string_in_quote <- <(!('"' / '\t' / '\r' / '\n') .)+> */
		func() bool {
			position238, tokenIndex238 := position, tokenIndex
			{
				position239 := position
				{
					position242, tokenIndex242 := position, tokenIndex
					{
						position243, tokenIndex243 := position, tokenIndex
						if buffer[position] != rune('"') {
							goto l244
						}
						position++
						goto l243
					l244:
						position, tokenIndex = position243, tokenIndex243
						if buffer[position] != rune('\t') {
							goto l245
						}
						position++
						goto l243
					l245:
						position, tokenIndex = position243, tokenIndex243
						if buffer[position] != rune('\r') {
							goto l246
						}
						position++
						goto l243
					l246:
						position, tokenIndex = position243, tokenIndex243
						if buffer[position] != rune('\n') {
							goto l242
						}
						position++
					}
				l243:
					goto l238
				l242:
					position, tokenIndex = position242, tokenIndex242
				}
				if !matchDot() {
					goto l238
				}
			l240:
				{
					position241, tokenIndex241 := position, tokenIndex
					{
						position247, tokenIndex247 := position, tokenIndex
						{
							position248, tokenIndex248 := position, tokenIndex
							if buffer[position] != rune('"') {
								goto l249
							}
							position++
							goto l248
						l249:
							position, tokenIndex = position248, tokenIndex248
							if buffer[position] != rune('\t') {
								goto l250
							}
							position++
							goto l248
						l250:
							position, tokenIndex = position248, tokenIndex248
							if buffer[position] != rune('\r') {
								goto l251
							}
							position++
							goto l248
						l251:
							position, tokenIndex = position248, tokenIndex248
							if buffer[position] != rune('\n') {
								goto l247
							}
							position++
						}
					l248:
						goto l241
					l247:
						position, tokenIndex = position247, tokenIndex247
					}
					if !matchDot() {
						goto l241
					}
					goto l240
				l241:
					position, tokenIndex = position241, tokenIndex241
				}
				add(rulestring_in_quote, position239)
			}
			return true
		l238:
			position, tokenIndex = position238, tokenIndex238
			return false
		},
		/* 31 cardinality <- <('0' / '1' / '*' / '+')> */
		func() bool {
			position252, tokenIndex252 := position, tokenIndex
			{
				position253 := position
				{
					position254, tokenIndex254 := position, tokenIndex
					if buffer[position] != rune('0') {
						goto l255
					}
					position++
					goto l254
				l255:
					position, tokenIndex = position254, tokenIndex254
					if buffer[position] != rune('1') {
						goto l256
					}
					position++
					goto l254
				l256:
					position, tokenIndex = position254, tokenIndex254
					if buffer[position] != rune('*') {
						goto l257
					}
					position++
					goto l254
				l257:
					position, tokenIndex = position254, tokenIndex254
					if buffer[position] != rune('+') {
						goto l252
					}
					position++
				}
			l254:
				add(rulecardinality, position253)
			}
			return true
		l252:
			position, tokenIndex = position252, tokenIndex252
			return false
		},
		nil,
		/* 34 Action0 <- <{p.Err(begin, buffer)}> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		/* 35 Action1 <- <{p.Err(begin, buffer)}> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 36 Action2 <- <{ p.ClearTableAndColumn() }> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 37 Action3 <- <{ p.AddTable(text) }> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 38 Action4 <- <{ p.AddColumn(text) }> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 39 Action5 <- <{ p.AddRelation() }> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 40 Action6 <- <{ p.SetRelationLeft(text) }> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 41 Action7 <- <{ p.SetCardinalityLeft(text)}> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 42 Action8 <- <{ p.SetRelationRight(text) }> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 43 Action9 <- <{ p.SetCardinalityRight(text)}> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 44 Action10 <- <{ p.AddTitleKeyValue() }> */
		func() bool {
			{
				add(ruleAction10, position)
			}
			return true
		},
		/* 45 Action11 <- <{ p.AddTableKeyValue() }> */
		func() bool {
			{
				add(ruleAction11, position)
			}
			return true
		},
		/* 46 Action12 <- <{ p.AddColumnKeyValue() }> */
		func() bool {
			{
				add(ruleAction12, position)
			}
			return true
		},
		/* 47 Action13 <- <{ p.AddRelationKeyValue() }> */
		func() bool {
			{
				add(ruleAction13, position)
			}
			return true
		},
		/* 48 Action14 <- <{ p.SetKey(text) }> */
		func() bool {
			{
				add(ruleAction14, position)
			}
			return true
		},
		/* 49 Action15 <- <{ p.SetValue(text) }> */
		func() bool {
			{
				add(ruleAction15, position)
			}
			return true
		},
		/* 50 Action16 <- <{ p.SetValue(text) }> */
		func() bool {
			{
				add(ruleAction16, position)
			}
			return true
		},
	}
	p.rules = _rules
}
