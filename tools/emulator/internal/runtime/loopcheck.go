package runtime

import (
	"fmt"
	"strings"
	"text/template"
	"text/template/parse"
)

// loopFinding is a database call inside a range loop.
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
	return fmt.Sprintf("%s: %s inside a range loop makes one database call per iteration "+
		"(YAGPDB allows %d per run, %d with premium). Fetch once before the loop, "+
		"or read many keys with one dbGetPattern.", where, f.Func, limitDB.normal, limitDB.premium)
}

// findLoopDBCalls finds database calls inside range loops. Each iteration is a separate
// call against YAGPDB's per-run database limit, so a loop over user input or a large
// list can run out of calls in production.
func findLoopDBCalls(tmpl *template.Template) []loopFinding {
	var findings []loopFinding
	for _, t := range tmpl.Templates() {
		if t.Tree == nil || t.Tree.Root == nil {
			continue
		}
		w := &loopWalker{tree: t.Tree}
		w.walk(t.Tree.Root, 0)
		findings = append(findings, w.findings...)
	}
	return findings
}

type loopWalker struct {
	tree     *parse.Tree
	findings []loopFinding
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
	case *parse.TemplateNode:
		w.pipe(n.Pipe, loopDepth)
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
		if id, ok := cmd.Args[0].(*parse.IdentifierNode); ok && loopDepth > 0 && isDBFunc(id.Ident) {
			w.findings = append(w.findings, loopFinding{Line: w.line(id), Func: id.Ident})
		}
		for _, arg := range cmd.Args {
			if sub, ok := arg.(*parse.PipeNode); ok {
				w.pipe(sub, loopDepth)
			}
		}
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
