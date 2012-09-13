package ggit

import (
    "io"
)

// Blob represents the deserialized version of a Git blob
// object.
type Blob struct {
    RawObject
    repo *Repository
}

func (b *Blob) String() string {
    p, _ := b.Payload()
    return string(p)
}

func (b *Blob) Type() ObjectType {
    return OBJECT_BLOB
}

func (b *Blob) WriteTo(w io.Writer) (n int, err error) {
    return io.WriteString(w, b.String())
}

// ToBlob converts a RawObject to a Blob object. The
// returned object is not associated with any repository
// by default.
func toBlob(obj *RawObject) (b *Blob, err error) {
    return &Blob{
        RawObject: *obj,
        repo:      nil,
    }
}
