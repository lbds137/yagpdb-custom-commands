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

// RoundTripExecData is what a delayed execCC does to its data (customcommands
// tmplextensions.go and bot.go handleDelayedRunCC): msgpack.Marshal when scheduled,
// msgpack.Unmarshal into an interface{} when the run starts. size is the encoded length.
func RoundTripExecData(v interface{}) (decoded interface{}, size int, err error) {
	encoded, err := msgpack.Marshal(v)
	if err != nil {
		return nil, 0, err
	}
	err = msgpack.Unmarshal(encoded, &decoded)
	return decoded, len(encoded), err
}
