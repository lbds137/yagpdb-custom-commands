package runtime

import (
	"fmt"
	template "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate/parse"
	"strings"
)

// loopFinding is a database call inside a loop.
type loopFinding struct {
	Line int
	Func string
}

// Message describes the finding; source names the template (may be empty).
func (f loopFinding) Message(source string) string {
	where := fmt.Sprintf("line %d", f.Line)
	if source != "" {
		where = fmt.Sprintf("%s:%d", source, f.Line)
	}
	return fmt.Sprintf("%s: %s inside a loop makes one database call per iteration "+
		"(YAGPDB allows %d per run, %d with premium). Fetch once before the loop, "+
		"or read many keys with one dbGetPattern.", where, f.Func, limitDB.normal, limitDB.premium)
}

// findLoopDBCalls finds database calls inside range and while loops. Each iteration is a separate
// call against YAGPDB's per-run database limit, so a loop over user input or a large
// list can run out of calls in production. A {{template}} call inside a loop counts
// when the named template makes database calls.
func findLoopDBCalls(tmpl *template.Template) []loopFinding {
	trees := map[string]*parse.Tree{}
	for _, t := range tmpl.Templates() {
		if t.Tree != nil && t.Tree.Root != nil {
			trees[t.Name()] = t.Tree
		}
	}

	var findings []loopFinding
	for _, t := range tmpl.Templates() {
		tree := trees[t.Name()]
		if tree == nil {
			continue
		}
		w := &loopWalker{tree: tree, trees: trees, resolveTemplates: true}
		w.walk(tree.Root, 0)
		findings = append(findings, w.findings...)
	}
	return findings
}

type loopWalker struct {
	tree     *parse.Tree
	trees    map[string]*parse.Tree // every defined template, for {{template}} calls
	findings []loopFinding
	called   []string // templates this tree calls

	// resolveTemplates flags {{template}} calls in loops; off while dbFuncIn scans a
	// template, which follows calls itself (with a guard against recursion)
	resolveTemplates bool
}

// dbFuncIn returns the first database function the named template (or a template it
// calls) uses, or "".
func (w *loopWalker) dbFuncIn(name string, seen map[string]bool) string {
	tree := w.trees[name]
	if tree == nil || seen[name] {
		return ""
	}
	seen[name] = true
	inner := &loopWalker{tree: tree, trees: w.trees}
	// Walking at depth 1 records every database call as a finding
	inner.walk(tree.Root, 1)
	if len(inner.findings) > 0 {
		return inner.findings[0].Func
	}
	for _, called := range inner.called {
		if f := w.dbFuncIn(called, seen); f != "" {
			return f
		}
	}
	return ""
}

func (w *loopWalker) walk(node parse.Node, loopDepth int) {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			w.walk(child, loopDepth)
		}
	case *parse.ActionNode:
		w.pipe(n.Pipe, loopDepth)
	case *parse.IfNode:
		w.pipe(n.Pipe, loopDepth)
		w.walk(n.List, loopDepth)
		w.walk(n.ElseList, loopDepth)
	case *parse.WithNode:
		w.pipe(n.Pipe, loopDepth)
		w.walk(n.List, loopDepth)
		w.walk(n.ElseList, loopDepth)
	case *parse.RangeNode:
		// The range expression itself runs once; only the body repeats.
		w.pipe(n.Pipe, loopDepth)
		w.walk(n.List, loopDepth+1)
		w.walk(n.ElseList, loopDepth)
	case *parse.WhileNode:
		// The condition is evaluated again before every iteration.
		w.pipe(n.Pipe, loopDepth+1)
		w.walk(n.List, loopDepth+1)
		w.walk(n.ElseList, loopDepth)
	case *parse.TryNode:
		w.walk(n.List, loopDepth)
		w.walk(n.CatchList, loopDepth)
	case *parse.ReturnNode:
		w.pipe(n.Pipe, loopDepth)
	case *parse.TemplateNode:
		w.pipe(n.Pipe, loopDepth)
		w.calledTemplate(n.Name, n, loopDepth)
	}
}

// calledTemplate records a call to a defined template, flagging it when it happens in
// a loop and the template makes database calls.
func (w *loopWalker) calledTemplate(name string, at parse.Node, loopDepth int) {
	w.called = append(w.called, name)
	if loopDepth > 0 && w.resolveTemplates {
		if f := w.dbFuncIn(name, map[string]bool{}); f != "" {
			w.findings = append(w.findings, loopFinding{
				Line: w.line(at),
				Func: fmt.Sprintf("template %q (which calls %s)", name, f),
			})
		}
	}
}

func (w *loopWalker) pipe(p *parse.PipeNode, loopDepth int) {
	if p == nil {
		return
	}
	for _, cmd := range p.Cmds {
		if len(cmd.Args) == 0 {
			continue
		}
		// execTemplate "name" runs a defined template, like {{template "name"}}
		if id, ok := cmd.Args[0].(*parse.IdentifierNode); ok && id.Ident == "execTemplate" && len(cmd.Args) > 1 {
			if name, ok := cmd.Args[1].(*parse.StringNode); ok {
				w.calledTemplate(name.Text, id, loopDepth)
			}
		}
		for _, arg := range cmd.Args {
			w.arg(arg, loopDepth)
		}
	}
}

// arg looks for calls in an argument: a function name, a parenthesized pipeline, or one
// with a field access such as (dbGet 0 "k").Value, which parses as a chain.
func (w *loopWalker) arg(node parse.Node, loopDepth int) {
	switch n := node.(type) {
	case *parse.IdentifierNode:
		// A function name is a call wherever it appears: {{dbGet ...}}, {{return dbCount}}
		if loopDepth > 0 && isDBFunc(n.Ident) {
			w.findings = append(w.findings, loopFinding{Line: w.line(n), Func: n.Ident})
		}
	case *parse.PipeNode:
		w.pipe(n, loopDepth)
	case *parse.ChainNode:
		w.arg(n.Node, loopDepth)
	}
}

func (w *loopWalker) line(n parse.Node) int {
	location, _ := w.tree.ErrorContext(n)
	// location is "name:line:col"
	parts := strings.Split(location, ":")
	if len(parts) < 3 {
		return 0
	}
	var line int
	fmt.Sscanf(parts[len(parts)-2], "%d", &line)
	return line
}

func isDBFunc(name string) bool {
	spec, ok := limitedFuncs[name]
	return ok && len(spec.limits) > 0 && spec.limits[0] == limitDB
}
