// Package main implements yagmin, the parser-based minifier for free-tier command
// size. A command on a free YAGPDB server may be at most 10,000 runes; yagmin drops
// the comments (except the header block, which carries the command's panel
// settings), renames every variable to a short name and re-prints the parse tree.
// The emulator's YAML suite runs against the minified copies (yagmin prove) to show
// the output is the same.
package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	template "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate/parse"
)

// nameAlphabet is the order short variable names are handed out in: $a..$z, $A..$Z,
// then $aa, $ab and so on.
const nameAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// commandFuncs is the func map a command is parsed with: exactly what the emulator's
// Engine builds (runtime.Engine.execute), so yagmin accepts and rejects the same
// sources the emulator does. Parsing only reads the names, so one map serves every
// file.
var commandFuncs template.FuncMap

func parseFuncs() template.FuncMap {
	if commandFuncs == nil {
		ctx := runtime.NewExecutionContext(1, state.NewMockDB(1))
		commandFuncs = runtime.NewEngine(ctx).BuildFuncMap()
	}
	return commandFuncs
}

// parseCommand parses source the way the emulator parses a command: the same func map,
// and no comment nodes kept (comments are dropped by the parse itself).
func parseCommand(name, source string) (*template.Template, error) {
	return template.New(name).Funcs(parseFuncs()).Parse(source)
}

// minify minifies one command: parse, rename the variables, re-print the tree, and
// check the output re-parses (the parser's undefined-variable checks make that a
// real test of the renaming, not just of the syntax). The header comment block, if
// the file starts with one, is kept verbatim: it holds the trigger and panel settings
// the emulator and the deploy tooling read (runtime.ReadTrigger), and it is part of
// what gets pasted into the panel.
func minify(name, source string) (string, error) {
	trees, err := commandTrees(name, source)
	if err != nil {
		return "", err
	}
	newRenamer(trees).apply(trees)
	stripWhitespaceNodes(trees)

	var b strings.Builder
	for _, t := range trees {
		if t.isMain {
			continue
		}
		fmt.Fprintf(&b, "{{define %q}}%s{{end}}", t.name, printNodes(t.root))
	}
	if main := mainTree(trees); main != nil {
		b.WriteString(printNodes(main.root))
	}
	out := b.String()
	if header := headerBlock(source); header != "" {
		out = header + "\n" + out
	}
	if _, err := parseCommand(name, out); err != nil {
		return "", fmt.Errorf("minified output does not re-parse (a minifier bug): %w", err)
	}
	return out, nil
}

// orderedTree is one of the file's parse trees: the command's own, or a {{define}}'s.
type orderedTree struct {
	name   string
	root   *parse.ListNode
	isMain bool
}

// commandTrees parses source and returns its trees in a deterministic order: the
// command's own tree first, then every {{define}} in name order. The order is also
// the "first appearance" order variable naming falls back to on ties.
func commandTrees(name, source string) ([]*orderedTree, error) {
	tmpl, err := parseCommand(name, source)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	var defs []*template.Template
	for _, t := range tmpl.Templates() {
		if t.Name() != tmpl.Name() && t.Tree != nil && t.Tree.Root != nil {
			defs = append(defs, t)
		}
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name() < defs[j].Name() })

	trees := make([]*orderedTree, 0, len(defs)+1)
	if tmpl.Tree != nil && tmpl.Tree.Root != nil {
		trees = append(trees, &orderedTree{name: tmpl.Name(), root: tmpl.Tree.Root, isMain: true})
	}
	for _, t := range defs {
		trees = append(trees, &orderedTree{name: t.Name(), root: t.Tree.Root})
	}
	return trees, nil
}

// mainTree returns the command's own tree.
func mainTree(trees []*orderedTree) *orderedTree {
	for _, t := range trees {
		if t.isMain {
			return t
		}
	}
	return nil
}

// headerRe matches the command's header comment block at the very start of a file
// ({{- /* Trigger type: ... */}}), with or without the trim markers.
var headerRe = regexp.MustCompile(`(?s)\A\s*\{\{-?\s*/\*.*?\*/\s*-?\}\}`)

// headerBlock returns the file's leading header comment block verbatim, or "".
func headerBlock(source string) string {
	return headerRe.FindString(source)
}

// walkNodes visits every VariableNode reachable from root, and nothing else: each
// node type that can hold a pipe, a list, command args, a chain or a declaration is
// followed, in source order. String literals, numbers and identifiers are leaves.
func walkNodes(root *parse.ListNode, visit func(*parse.VariableNode)) {
	var walk func(parse.Node)
	walk = func(n parse.Node) {
		switch node := n.(type) {
		case *parse.ListNode:
			if node == nil {
				return
			}
			for _, child := range node.Nodes {
				walk(child)
			}
		case *parse.ActionNode:
			walkPipe(node.Pipe, walk)
		case *parse.IfNode:
			walkBranch(&node.BranchNode, walk)
		case *parse.WithNode:
			walkBranch(&node.BranchNode, walk)
		case *parse.RangeNode:
			walkBranch(&node.BranchNode, walk)
		case *parse.WhileNode:
			walkBranch(&node.BranchNode, walk)
		case *parse.TryNode:
			walk(node.List)
			walk(node.CatchList)
		case *parse.ReturnNode:
			walkPipe(node.Pipe, walk)
		case *parse.TemplateNode:
			walkPipe(node.Pipe, walk)
		case *parse.CommandNode:
			for _, arg := range node.Args {
				walk(arg)
			}
		case *parse.PipeNode:
			walkPipe(node, walk)
		case *parse.ChainNode:
			walk(node.Node)
		case *parse.VariableNode:
			visit(node)
		}
	}
	walk(root)
}

// walkPipe walks a pipe's declarations and commands; Decl and uses are the same
// VariableNodes, so both are visited.
func walkPipe(p *parse.PipeNode, walk func(parse.Node)) {
	if p == nil {
		return
	}
	for _, decl := range p.Decl {
		walk(decl)
	}
	for _, cmd := range p.Cmds {
		walk(cmd)
	}
}

func walkBranch(b *parse.BranchNode, walk func(parse.Node)) {
	walkPipe(b.Pipe, walk)
	walk(b.List)
	walk(b.ElseList)
}

// renamer renames a file's variables with one injective map: the most-used variable
// gets $a, the next $b and so on, ties broken by first appearance. Field chains
// ($x.Field.Chan) keep their fields; only the first identifier is renamed. The bare
// $ (and $.Field) is never renamed.
type renamer struct {
	short map[string]string // original first identifier -> short name
}

// newRenamer counts the variables across every tree (in commandTrees order) and
// builds the name map.
func newRenamer(trees []*orderedTree) *renamer {
	count := map[string]int{}
	first := map[string]int{}
	next := 0
	for _, t := range trees {
		walkNodes(t.root, func(v *parse.VariableNode) {
			name := v.Ident[0]
			if name == "$" {
				return
			}
			if _, seen := first[name]; !seen {
				first[name] = next
				next++
			}
			count[name]++
		})
	}

	names := make([]string, 0, len(count))
	for name := range count {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if count[names[i]] != count[names[j]] {
			return count[names[i]] > count[names[j]]
		}
		return first[names[i]] < first[names[j]]
	})

	short := make(map[string]string, len(names))
	for i, name := range names {
		short[name] = "$" + shortName(i)
	}
	return &renamer{short: short}
}

// shortName returns the i-th short identifier (without the $): a, b, ..., z, A, ...,
// Z, aa, ab, ...
func shortName(i int) string {
	base := len(nameAlphabet)
	if i < base {
		return nameAlphabet[i : i+1]
	}
	// two letters: 52*52 slots, "aa", "ab", ..., "aZ", "ba", ...
	i -= base
	return nameAlphabet[i/base:i/base+1] + nameAlphabet[i%base:i%base+1]
}

// stripWhitespaceNodes removes every text node that is entirely whitespace. Those
// nodes are the indentation and newlines between actions; they only add whitespace
// to the command's output. The equivalence proof (yagmin prove) is the oracle for
// that: if any test's output turns out to depend on them, it shows up as a real
// difference. Text nodes with any non-whitespace content are kept verbatim, and
// string literals live in actions, untouched by this pass.
func stripWhitespaceNodes(trees []*orderedTree) {
	for _, t := range trees {
		stripList(t.root)
	}
}

// strip follows a node that can hold further lists, pipes or command arguments.
func strip(n parse.Node) {
	switch node := n.(type) {
	case *parse.ActionNode:
		stripPipe(node.Pipe)
	case *parse.IfNode:
		stripBranch(&node.BranchNode)
	case *parse.WithNode:
		stripBranch(&node.BranchNode)
	case *parse.RangeNode:
		stripBranch(&node.BranchNode)
	case *parse.WhileNode:
		stripBranch(&node.BranchNode)
	case *parse.TryNode:
		stripList(node.List)
		stripList(node.CatchList)
	case *parse.ReturnNode:
		stripPipe(node.Pipe)
	case *parse.TemplateNode:
		stripPipe(node.Pipe)
	case *parse.CommandNode:
		for _, arg := range node.Args {
			stripArg(arg)
		}
	case *parse.PipeNode:
		stripPipe(node)
	case *parse.ChainNode:
		stripArg(node)
	}
}

func stripPipeCmd(c *parse.CommandNode) {
	for _, arg := range c.Args {
		stripArg(arg)
	}
}

// stripArg follows the node types that can hold further lists (a parenthesized
// pipeline argument, or a chain on one).
func stripArg(arg parse.Node) {
	switch a := arg.(type) {
	case *parse.PipeNode:
		stripPipe(a)
	case *parse.ChainNode:
		stripArg(a.Node)
	}
}

// stripPipe walks a pipe's commands' arguments (declarations hold no lists).
func stripPipe(p *parse.PipeNode) {
	if p == nil {
		return
	}
	for _, cmd := range p.Cmds {
		stripPipeCmd(cmd)
	}
}

func stripBranch(b *parse.BranchNode) {
	stripPipe(b.Pipe)
	stripList(b.List)
	stripList(b.ElseList)
}

// stripList walks a list, dropping whitespace-only text nodes on the way.
func stripList(n *parse.ListNode) {
	if n == nil {
		return
	}
	kept := n.Nodes[:0]
	for _, child := range n.Nodes {
		if t, ok := child.(*parse.TextNode); ok && len(t.Text) > 0 &&
			len(strings.TrimSpace(string(t.Text))) == 0 {
			continue
		}
		strip(child)
		kept = append(kept, child)
	}
	n.Nodes = kept
}

// printNodes prints a list of nodes. It mirrors the nodes' own String methods
// everywhere except one: parse's PipeNode.String always prints ":=" and loses the
// IsAssign flag ({{$x = v}} would re-print as {{$x := v}}, which shadows instead of
// assigning). Every node without a pipe prints with its own String, exactly as
// parse wrote it.
func printNodes(n *parse.ListNode) string {
	var b strings.Builder
	for _, child := range n.Nodes {
		switch node := child.(type) {
		case *parse.ActionNode:
			fmt.Fprintf(&b, "{{%s}}", printPipe(node.Pipe))
		case *parse.IfNode:
			fmt.Fprintf(&b, "%s", printBranch("if", &node.BranchNode))
		case *parse.WithNode:
			fmt.Fprintf(&b, "%s", printBranch("with", &node.BranchNode))
		case *parse.RangeNode:
			fmt.Fprintf(&b, "%s", printBranch("range", &node.BranchNode))
		case *parse.WhileNode:
			fmt.Fprintf(&b, "%s", printBranch("while", &node.BranchNode))
		case *parse.TryNode:
			fmt.Fprintf(&b, "{{try}}%s{{catch}}%s{{end}}",
				printNodes(node.List), printNodes(node.CatchList))
		case *parse.ReturnNode:
			if node.Pipe != nil {
				fmt.Fprintf(&b, "{{return %s}}", printPipe(node.Pipe))
			} else {
				b.WriteString("{{return}}")
			}
		case *parse.TemplateNode:
			if node.Pipe != nil {
				fmt.Fprintf(&b, "{{template %q %s}}", node.Name, printPipe(node.Pipe))
			} else {
				fmt.Fprintf(&b, "{{template %q}}", node.Name)
			}
		default:
			// Text, variables, fields, identifiers, literals, break and continue:
			// no pipes inside, so the node's own String is exact
			fmt.Fprintf(&b, "%s", child)
		}
	}
	return b.String()
}

// printBranch prints an if/with/range/while, with its else branch if it has one.
func printBranch(name string, b *parse.BranchNode) string {
	if b.ElseList != nil {
		return fmt.Sprintf("{{%s %s}}%s{{else}}%s{{end}}", name, printPipe(b.Pipe),
			printNodes(b.List), printNodes(b.ElseList))
	}
	return fmt.Sprintf("{{%s %s}}%s{{end}}", name, printPipe(b.Pipe), printNodes(b.List))
}

// printPipe prints a pipe's declarations and commands, keeping "=" where the source
// assigned (the one thing PipeNode.String gets wrong).
func printPipe(p *parse.PipeNode) string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	if len(p.Decl) > 0 {
		for i, v := range p.Decl {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(v.String())
		}
		if p.IsAssign {
			b.WriteString(" = ")
		} else {
			b.WriteString(" := ")
		}
	}
	for i, c := range p.Cmds {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(printCommand(c))
	}
	return b.String()
}

// printCommand prints a command's arguments; a nested pipeline argument is
// parenthesized, as CommandNode.String does.
func printCommand(c *parse.CommandNode) string {
	var b strings.Builder
	for i, arg := range c.Args {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(printArg(arg))
	}
	return b.String()
}

// printArg prints a command argument: a pipeline (parenthesized), a chain on one
// (the pipeline parenthesized, then the fields), or any other node's own String.
func printArg(arg parse.Node) string {
	switch a := arg.(type) {
	case *parse.PipeNode:
		return "(" + printPipe(a) + ")"
	case *parse.ChainNode:
		inner := printArg(a.Node)
		s := inner
		if _, ok := a.Node.(*parse.PipeNode); ok {
			s = "(" + inner + ")"
		}
		for _, field := range a.Field {
			s += "." + field
		}
		return s
	default:
		return a.String()
	}
}
func (r *renamer) apply(trees []*orderedTree) {
	for _, t := range trees {
		walkNodes(t.root, func(v *parse.VariableNode) {
			if name := v.Ident[0]; name != "$" {
				short, ok := r.short[name]
				if !ok {
					// unreachable: apply walks the same nodes newRenamer counted
					panic("yagmin: no short name for " + name)
				}
				v.Ident[0] = short
			}
		})
	}
}
