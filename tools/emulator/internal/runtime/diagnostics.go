package runtime

import "fmt"

// Diagnostic kinds.
const (
	KindLimit  = "limit"   // a YAGPDB execution limit was reached
	KindLoopDB = "loop-db" // a database call runs once per loop iteration
	KindSchema = "schema"  // a stored value does not match the schema
	KindExecCC = "execcc"  // a command run by execCC failed
	KindRole   = "role"    // a role was assumed to exist
	KindDB     = "db"      // a query production may fail where the mock doesn't
)

// Diagnostic is a warning found while checking or running a template.
type Diagnostic struct {
	Kind    string
	Message string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("[%s] %s", d.Kind, d.Message)
}

// Warn records a diagnostic.
func (ctx *ExecutionContext) Warn(kind, format string, args ...interface{}) {
	ctx.Diagnostics = append(ctx.Diagnostics, Diagnostic{Kind: kind, Message: fmt.Sprintf(format, args...)})
}
