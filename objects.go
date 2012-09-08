package ggit

import (
        "crypto/sha1"
        "errors"
        "hash"
        "strconv"
        "strings"
)

type ObjectType int

// the types of objects
const (
        OBJECT_BLOB ObjectType = iota
        OBJECT_TREE
        OBJECT_COMMIT
        OBJECT_TAG
)

// raw (but uncompressed) data for a
// git object that contains the header;
type RawObject struct {
        bytes []byte
        pInx int64 // start of payload bytes 
}

type ObjectHeader struct {
        Type    ObjectType
        Size    int
}

func toObjectType(typeStr string) (otype ObjectType, err error) {
        switch typeStr {
        case "blob":
                return OBJECT_BLOB, nil
        case "tree":
                return OBJECT_TREE, nil
        case "tag":
                return OBJECT_TAG, nil
        case "commit":
                return OBJECT_COMMIT, nil
        }
        return 0, errors.New("unknown object type")
}

// TODO: this function probably shouldn't even exist; we just
// need a good header parser below
func toObjectHeader(header string) (h *ObjectHeader, err error) {
		// TODO: this needs to be all sorts of fixed; what if there
		// is leading whitespace?
		header = strings.Trim(header, "\000")
        toks := strings.Split(header, " ")
        if len(toks) != 2 {
                return nil, errors.New("bad object header")
        }
        typeStr, sizeStr := toks[0], toks[1]
        otype, err := toObjectType(typeStr)
        if err != nil {
                return
        }

        osize, err := strconv.Atoi(sizeStr)
        if err != nil {
                return nil, errors.New("bad object size")
        }

        return &ObjectHeader{otype, osize}, nil
}

// interface for hashable objects
type Hashable interface {
        Bytes() []byte
}

// parses the header from the raw data
func (o *RawObject) Header() (h *ObjectHeader, err error) {
	if len(o.bytes) < 1 {
		return nil, errors.New("no data bytes")
	}
	var typeStr, sizeStr string
	typeStr, sizeStr, o.pInx = parseHeader(o.bytes)
	if o.pInx <= 0 {
			return nil, errors.New("bad header")
	}
	otype, err := toObjectType(typeStr)
	if err != nil {
		return
	}
	osize, err := strconv.Atoi(sizeStr)
	if err != nil {
	    return nil, errors.New("bad object size")
	}
    return &ObjectHeader{otype, osize}, nil
}

func parseHeader(b []byte) (typeStr, sizeStr string, pInx int64) {
	const MAX_HEADER_SZ = 32
	var i, j int64
	for i = 0; i < MAX_HEADER_SZ; i++ {
		if b[i] == ' ' {
			typeStr = string(b[:i])
			for j = i; j < MAX_HEADER_SZ; j++ {
				if b[j] == '\000' {
					pInx = j
					sizeStr = string(b[i+1:j])
					return
				}
			}
		}
	}
	return
}

// returns the headerless payload of the object
func (o *RawObject) Payload() (bts []byte, err error) {
	if o.pInx <= 0 {
		return nil, errors.New("bad header")
	}
	return o.bytes[o.pInx+1:], nil
}

func (o *RawObject) Write(b []byte) (n int, err error) {
        if o.bytes == nil {
                o.bytes = make([]byte, len(b))
                return copy(o.bytes, b), nil
        }
        return 0, errors.New("object already has data")
}

// returns the raw byte representation of
// the object
func (o *RawObject) Bytes() []byte {
        return o.bytes
}


// the hash object used to build
// hashes of our objects
var sha hash.Hash = sha1.New()

// produce a hash for any object that
// can be construed as a bunch of bytes
func Hash(h Hashable) (o *ObjectId) {
        sha.Reset()
        sha.Write(h.Bytes())
        return NewObjectIdFromHash(sha)
}

type Blob struct {
        RawObject
}
