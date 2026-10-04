package main

import (
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate/parse"
)

// Whitespace removal that cannot change the response.
//
// YAGPDB trims the whole response before sending it (customcommands/bot.go:764 and
// common/templates/context.go:565, both `out = strings.TrimSpace(out)`), so a
// whitespace-only text node W changes the response only when one run prints
// non-whitespace both before and after it. W is dropped when that cannot happen:
// nothing that can run before W prints non-whitespace, or nothing that can run after
// it does. "Can run before/after" is taken in document order, with one exception: a
// loop runs its body again after W, so for W inside a loop the whole outermost loop
// counts as both before and after it.
//
// A "printer" is a node that may write non-whitespace to the response: a text node
// with content; an action that prints its value (no `:=`/`=`) unless the value is
// known to print as "" (silentFuncs, silentMethods); a {{template}} call or an
// execTemplate call (which runs the define on the same output, lib/template
// exec.go:1030-1041) whose define may print. Pipes used as conditions print nothing
// themselves, but an execTemplate inside one does.
//
// A define's whitespace lands wherever the define is called, from the main tree or
// from other defines, so it is dropped only when no tree prints at all (then the whole
// response is whitespace and trims to ""). sendTemplate and sendTemplateDM send a
// define's output as a message of its own (context_funcs.go:276-287), not through the
// trimmed response, so a file using either keeps every define's whitespace.
//
// The main template can be called like a define: YAGPDB names it "CC #<id>"
// (customcommands/bot.go:722), so {{template "CC #5"}}, an execTemplate with a
// computed name, or a sendTemplate naming it runs the main tree nested. When any
// template, execTemplate or sendTemplate name in the file is computed or isn't one of
// the file's defines, the main tree is treated like a define: its whitespace drops
// only when nothing prints anywhere, and not at all when a sendTemplate may send it.

// silentFuncs are the functions whose value prints as "" on every path that reaches
// the print: each returns the empty string, or an error, which stops the action before
// it prints. Vendor citations are into vendor/yagpdb.
var silentFuncs = map[string]bool{
	// (any, error): "" on every return; registered with returnID=false
	// (common/templates/context.go:828), so the message id is never returned
	// (context_funcs.go:437-440)
	"sendMessage": true,
	// (interface{}, error): "" (context_interactions.go:366-369); registered with
	// returnID=false (context_interactions.go:24)
	"sendResponse": true,
	// (interface{}, error): "" (context_interactions.go:303-304)
	"sendModal": true,
	// (interface{}, error): "" (context_funcs.go:475)
	"editMessage": true,
	// string: "" (context_funcs.go:900, 916)
	"deleteMessage": true,
	// string: "" (context_funcs.go:891, 894, via tmplDelMessage)
	"deleteTrigger": true,
	// string: "" (context_funcs.go:886)
	"deleteResponse": true,
	// (string, error): "" on every return (customcommands/tmplextensions.go:180-282)
	"execCC": true,
	// (string, error): dbSet calls dbSetExpire (tmplextensions.go:405), whose returns
	// are all "" (tmplextensions.go:412-449)
	"dbSet":       true,
	"dbSetExpire": true,
	// (interface{}, error): "" (tmplextensions.go:564, 572)
	"dbDel": true,
	// (interface{}, error): "" (tmplextensions.go:579, 586)
	"dbDelById": true,
}

// silentMethods are the methods that print as "" on the containers constructorFuncs
// build: Dict.Set/Del (common/templates/context.go:1109-1120, 1135-1138), SDict.Set/Del
// (context.go:1169-1177, 1183-1186) and Slice.Set (context.go:1215-1226; Slice has no
// Del, so that call errors instead). The call is silent only on a variable whose
// every binding in the file is one of those constructors: on any other receiver it
// is a printer. A nil receiver (a missing dbGet's .Value, an unset variable) does not
// error: the pipeline turns nil into an invalid Value (lib/template exec.go:648-649),
// a field of an invalid receiver evaluates to the zero Value without error
// (exec.go:795-800), and that prints "<no value>" (exec.go:1309-1310). And the modal
// builder's Set returns the builder (context.go:1298), which prints too.
var silentMethods = map[string]bool{"Set": true, "Del": true}

// constructorFuncs build the containers silentMethods are known on: dict
// (context.go:118 -> general.go:23), sdict (context.go:119 -> general.go:62) and cslice
// (context.go:122 -> general.go:220).
var constructorFuncs = map[string]bool{"dict": true, "sdict": true, "cslice": true}

// nestedSenders execute a template outside the response (see the package comment),
// with the template's name at the given argument index.
var nestedSenders = map[string]int{"sendTemplate": 1, "sendTemplateDM": 0}

// allDefines marks a call whose template name isn't a literal: it may run any tree.
const allDefines = "*"

// whitespaceAnalysis holds what the removal pass needs to know about a file.
type whitespaceAnalysis struct {
	trees []*orderedTree
	main  *orderedTree
	// containerVars are the variables whose every binding is a constructorFuncs call
	containerVars map[string]bool
	// prints says, per tree name, whether running the tree may print non-whitespace
	prints map[string]bool
	// anyPrinter is true when some tree has a printer of its own
	anyPrinter bool
	// keepDefines is true when a nested sender may send a define's output
	keepDefines bool
	// mainReentrant is true when a template or execTemplate call may run the main
	// tree (a computed name, or one that isn't a define of the file)
	mainReentrant bool
	// keepMain is true when a nested sender may send the main tree's output
	keepMain bool
}

// dropWhitespace removes every whitespace-only text node that cannot change the
// response.
func dropWhitespace(trees []*orderedTree) {
	a := analyzeWhitespace(trees)
	nothingPrints := !a.anyPrinter && !a.keepDefines
	if a.main != nil {
		switch {
		case a.keepMain:
		case a.mainReentrant:
			if nothingPrints {
				dropAllWhitespace(a.main.root)
			}
		default:
			a.dropFromMain(a.main.root)
		}
	}
	if nothingPrints {
		for _, t := range trees {
			if !t.isMain {
				dropAllWhitespace(t.root)
			}
		}
	}
}

// analyzeWhitespace computes the per-file facts: container variables, nested senders,
// main re-entry, and which trees may print (own printers, closed over template and
// execTemplate calls).
func analyzeWhitespace(trees []*orderedTree) *whitespaceAnalysis {
	a := &whitespaceAnalysis{trees: trees, main: mainTree(trees), prints: map[string]bool{}}
	a.containerVars = containerVariables(trees)
	defines := map[string]bool{}
	for _, t := range trees {
		if !t.isMain {
			defines[t.name] = true
		}
	}

	own := map[string]bool{}
	callees := map[string]map[string]bool{}
	for _, t := range trees {
		calls := map[string]bool{}
		eachNode(t.root, func(n parse.Node) {
			switch node := n.(type) {
			case *parse.TextNode:
				if len(node.Text) > 0 && !isWhitespaceText(node) {
					own[t.name] = true
				}
			case *parse.ActionNode:
				if len(node.Pipe.Decl) == 0 && !a.silentPipe(node.Pipe) {
					own[t.name] = true
				}
			case *parse.TemplateNode:
				calls[node.Name] = true
				if !defines[node.Name] {
					a.mainReentrant = true
				}
			case *parse.CommandNode:
				if callee, ok := execTemplateCallee(node); ok {
					calls[callee] = true
					if !defines[callee] {
						a.mainReentrant = true
					}
				}
				if name, ok := nestedSenderCallee(node); ok {
					a.keepDefines = true
					if !defines[name] {
						a.keepMain = true
					}
				}
			}
		})
		callees[t.name] = calls
	}
	for name := range own {
		a.prints[name] = true
		a.anyPrinter = true
	}
	// close over calls: a tree prints when any define it may run prints
	for changed := true; changed; {
		changed = false
		for _, t := range trees {
			if a.prints[t.name] {
				continue
			}
			for callee := range callees[t.name] {
				if a.calleePrints(callee) {
					a.prints[t.name] = true
					changed = true
					break
				}
			}
		}
	}
	return a
}

// calleePrints says whether a call to the named template may print. A define's name
// answers for that define; any other name (allDefines, or a name that isn't a define:
// it may be the main template's own "CC #<id>") answers for every tree, main included.
func (a *whitespaceAnalysis) calleePrints(name string) bool {
	if name != allDefines && a.isDefine(name) {
		return a.prints[name]
	}
	for _, t := range a.trees {
		if a.prints[t.name] {
			return true
		}
	}
	return false
}

func (a *whitespaceAnalysis) isDefine(name string) bool {
	for _, t := range a.trees {
		if !t.isMain && t.name == name {
			return true
		}
	}
	return false
}

// silentPipe reports whether an action's pipe is known to print "": a single command
// whose function is in silentFuncs, or a silentMethods call on a constructor-bound
// variable (see silentMethods for why no other receiver qualifies).
func (a *whitespaceAnalysis) silentPipe(p *parse.PipeNode) bool {
	if p == nil || len(p.Cmds) != 1 || len(p.Cmds[0].Args) == 0 {
		return false
	}
	switch head := p.Cmds[0].Args[0].(type) {
	case *parse.IdentifierNode:
		return silentFuncs[head.Ident]
	case *parse.VariableNode:
		return len(head.Ident) == 2 && silentMethods[head.Ident[1]] &&
			a.containerVars[head.Ident[0]]
	}
	return false
}

// nestedSenderCallee returns the template a sendTemplate or sendTemplateDM command
// runs: its literal name, or allDefines when the name is computed or missing.
func nestedSenderCallee(c *parse.CommandNode) (string, bool) {
	if len(c.Args) == 0 {
		return "", false
	}
	head, ok := c.Args[0].(*parse.IdentifierNode)
	if !ok {
		return "", false
	}
	index, ok := nestedSenders[head.Ident]
	if !ok {
		return "", false
	}
	if len(c.Args) > index+1 {
		if name, ok := c.Args[index+1].(*parse.StringNode); ok {
			return name.Text, true
		}
	}
	return allDefines, true
}

// containerVariables returns the variables whose every binding in the file is a
// constructorFuncs call. A range declaration binds elements, never the pipe's value,
// so it disqualifies its variables. `$` is never one: every tree binds it to its dot
// before any line runs (lib/template exec.go:1035, and the top level), a binding no
// `$ = sdict` in the file can rule out.
func containerVariables(trees []*orderedTree) map[string]bool {
	bound := map[string]bool{"$": true}      // seen a binding at all
	container := map[string]bool{"$": false} // every binding so far is a constructor
	bind := func(p *parse.PipeNode, isConstructor bool) {
		if p == nil {
			return
		}
		for _, v := range p.Decl {
			name := v.Ident[0]
			if !bound[name] {
				bound[name] = true
				container[name] = true
			}
			if !isConstructor {
				container[name] = false
			}
		}
	}
	for _, t := range trees {
		eachNode(t.root, func(n parse.Node) {
			switch node := n.(type) {
			case *parse.RangeNode:
				bind(node.Pipe, false)
			case *parse.PipeNode:
				if len(node.Decl) > 0 {
					bind(node, isConstructorPipe(node))
				}
			}
		})
	}
	// a RangeNode's pipe is visited twice (once as the range, once as a pipe); the
	// range visit comes first and already turned its variables off
	return container
}

// isConstructorPipe reports whether a pipe's value is a constructorFuncs call: its
// last command's function, through parentheses.
func isConstructorPipe(p *parse.PipeNode) bool {
	if p == nil || len(p.Cmds) == 0 {
		return false
	}
	last := p.Cmds[len(p.Cmds)-1]
	if len(last.Args) == 0 {
		return false
	}
	switch head := last.Args[0].(type) {
	case *parse.IdentifierNode:
		return constructorFuncs[head.Ident]
	case *parse.PipeNode:
		return len(last.Args) == 1 && isConstructorPipe(head)
	}
	return false
}

// execTemplateCallee returns the define an execTemplate command runs: its literal
// name, or allDefines when the name is computed.
func execTemplateCallee(c *parse.CommandNode) (string, bool) {
	if len(c.Args) == 0 {
		return "", false
	}
	head, ok := c.Args[0].(*parse.IdentifierNode)
	if !ok || head.Ident != "execTemplate" {
		return "", false
	}
	if len(c.Args) > 1 {
		if name, ok := c.Args[1].(*parse.StringNode); ok {
			return name.Text, true
		}
	}
	return allDefines, true
}

// pipePrints reports whether evaluating a pipe may print: it holds an execTemplate call
// on a define that prints.
func (a *whitespaceAnalysis) pipePrints(p *parse.PipeNode) bool {
	if p == nil {
		return false
	}
	prints := false
	eachNode(p, func(n parse.Node) {
		if c, ok := n.(*parse.CommandNode); ok {
			if callee, ok := execTemplateCallee(c); ok && a.calleePrints(callee) {
				prints = true
			}
		}
	})
	return prints
}

// whitespaceItem is one whitespace-only text node of the main tree, with its place in
// the document-order event sequence and the extent of its outermost enclosing loop.
type whitespaceItem struct {
	list      *parse.ListNode
	index     int
	pos       int // this node's event index
	loopStart int // event range of the outermost loop around it, or -1
	loopEnd   int
}

// linearizer walks the main tree in document order, recording every printer as an
// event and every whitespace node as an item.
type linearizer struct {
	a      *whitespaceAnalysis
	events []bool // true: a printer
	items  []*whitespaceItem
	inLoop bool
}

func (l *linearizer) printer(prints bool) {
	if prints {
		l.events = append(l.events, true)
	}
}

func (l *linearizer) list(n *parse.ListNode) {
	if n == nil {
		return
	}
	for i, child := range n.Nodes {
		switch node := child.(type) {
		case *parse.TextNode:
			if isWhitespaceText(node) {
				l.items = append(l.items, &whitespaceItem{
					list: n, index: i, pos: len(l.events), loopStart: -1, loopEnd: -1})
				l.events = append(l.events, false)
			} else if len(node.Text) > 0 {
				l.printer(true)
			}
		case *parse.ActionNode:
			l.printer(l.a.pipePrints(node.Pipe) ||
				(len(node.Pipe.Decl) == 0 && !l.a.silentPipe(node.Pipe)))
		case *parse.IfNode:
			l.branch(&node.BranchNode)
		case *parse.WithNode:
			l.branch(&node.BranchNode)
		case *parse.RangeNode:
			l.loop(&node.BranchNode)
		case *parse.WhileNode:
			l.loop(&node.BranchNode)
		case *parse.TryNode:
			l.list(node.List)
			l.list(node.CatchList)
		case *parse.ReturnNode:
			l.printer(l.a.pipePrints(node.Pipe))
		case *parse.TemplateNode:
			l.printer(l.a.pipePrints(node.Pipe) || l.a.calleePrints(node.Name))
		}
	}
}

func (l *linearizer) branch(b *parse.BranchNode) {
	l.printer(l.a.pipePrints(b.Pipe))
	l.list(b.List)
	l.list(b.ElseList)
}

// loop records a range or while: the outermost one marks every whitespace item inside
// it with the loop's whole event range, since the body runs again after the item.
func (l *linearizer) loop(b *parse.BranchNode) {
	if l.inLoop {
		l.branch(b)
		return
	}
	l.inLoop = true
	start, from := len(l.events), len(l.items)
	l.branch(b)
	end := len(l.events)
	for _, item := range l.items[from:] {
		item.loopStart, item.loopEnd = start, end
	}
	l.inLoop = false
}

// dropFromMain decides every whitespace node of the main tree and removes the
// droppable ones.
func (a *whitespaceAnalysis) dropFromMain(root *parse.ListNode) {
	l := &linearizer{a: a}
	l.list(root)

	// before[i] = printers among events[0:i]
	before := make([]int, len(l.events)+1)
	for i, printer := range l.events {
		before[i+1] = before[i]
		if printer {
			before[i+1]++
		}
	}
	total := before[len(l.events)]

	drop := map[*parse.ListNode]map[int]bool{}
	for _, item := range l.items {
		start, end := item.pos, item.pos+1
		if item.loopStart >= 0 {
			start, end = item.loopStart, item.loopEnd
		}
		inside := before[end] - before[start]
		if inside == 0 && (before[start] == 0 || total-before[end] == 0) {
			if drop[item.list] == nil {
				drop[item.list] = map[int]bool{}
			}
			drop[item.list][item.index] = true
		}
	}
	for list, indexes := range drop {
		kept := list.Nodes[:0]
		for i, node := range list.Nodes {
			if !indexes[i] {
				kept = append(kept, node)
			}
		}
		list.Nodes = kept
	}
}

// dropAllWhitespace removes every whitespace-only text node under a list.
func dropAllWhitespace(n *parse.ListNode) {
	if n == nil {
		return
	}
	kept := n.Nodes[:0]
	for _, child := range n.Nodes {
		if t, ok := child.(*parse.TextNode); ok && isWhitespaceText(t) {
			continue
		}
		switch node := child.(type) {
		case *parse.IfNode:
			dropAllWhitespace(node.List)
			dropAllWhitespace(node.ElseList)
		case *parse.WithNode:
			dropAllWhitespace(node.List)
			dropAllWhitespace(node.ElseList)
		case *parse.RangeNode:
			dropAllWhitespace(node.List)
			dropAllWhitespace(node.ElseList)
		case *parse.WhileNode:
			dropAllWhitespace(node.List)
			dropAllWhitespace(node.ElseList)
		case *parse.TryNode:
			dropAllWhitespace(node.List)
			dropAllWhitespace(node.CatchList)
		}
		kept = append(kept, child)
	}
	n.Nodes = kept
}

// isWhitespaceText reports whether a text node is non-empty and all whitespace (the
// TrimSpace sense, which is what YAGPDB trims by).
func isWhitespaceText(t *parse.TextNode) bool {
	return len(t.Text) > 0 && strings.TrimSpace(string(t.Text)) == ""
}

// eachNode visits every node under root in document order: lists, control nodes,
// pipes (declarations and commands), command arguments, chains and leaves.
func eachNode(root parse.Node, visit func(parse.Node)) {
	var walk func(parse.Node)
	walk = func(n parse.Node) {
		if n == nil {
			return
		}
		switch node := n.(type) {
		case *parse.ListNode:
			if node == nil {
				return
			}
			visit(node)
			for _, child := range node.Nodes {
				walk(child)
			}
		case *parse.PipeNode:
			if node == nil {
				return
			}
			visit(node)
			for _, decl := range node.Decl {
				walk(decl)
			}
			for _, cmd := range node.Cmds {
				walk(cmd)
			}
		case *parse.ActionNode:
			visit(node)
			walk(node.Pipe)
		case *parse.IfNode:
			visit(node)
			walkBranchNodes(&node.BranchNode, walk)
		case *parse.WithNode:
			visit(node)
			walkBranchNodes(&node.BranchNode, walk)
		case *parse.RangeNode:
			visit(node)
			walkBranchNodes(&node.BranchNode, walk)
		case *parse.WhileNode:
			visit(node)
			walkBranchNodes(&node.BranchNode, walk)
		case *parse.TryNode:
			visit(node)
			walk(node.List)
			walk(node.CatchList)
		case *parse.ReturnNode:
			visit(node)
			walk(node.Pipe)
		case *parse.TemplateNode:
			visit(node)
			walk(node.Pipe)
		case *parse.CommandNode:
			visit(node)
			for _, arg := range node.Args {
				walk(arg)
			}
		case *parse.ChainNode:
			visit(node)
			walk(node.Node)
		default:
			visit(node)
		}
	}
	walk(root)
}

func walkBranchNodes(b *parse.BranchNode, walk func(parse.Node)) {
	walk(b.Pipe)
	walk(b.List)
	walk(b.ElseList)
}
