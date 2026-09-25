package state

import (
	"bytes"

	"github.com/vmihailenco/msgpack"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// YAGPDB registers its template types so they come back as themselves
// (common/templates/context.go); the extensions also count towards a value's size.
func init() {
	msgpack.RegisterExt(1, (*types.SDict)(nil))
	msgpack.RegisterExt(2, (*types.Dict)(nil))
	msgpack.RegisterExt(3, (*types.Slice)(nil))
}

// serializeValue is YAGPDB's (customcommands/tmplextensions.go): the value as stored,
// msgpack-encoded, at most 100000 bytes (yagstd.LimitWriter). A bigger value fails with
// "short write".
func serializeValue(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	enc := msgpack.NewEncoder(yagstd.LimitWriter(&b, 100000))
	err := enc.Encode(v)
	return b.Bytes(), err
}
