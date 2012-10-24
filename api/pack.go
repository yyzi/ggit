package api

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"sort"
	"strconv"
)

const (
	PackSignature    = "PACK"    //0x5041434b
	PackIdxSignature = "\377tOc" //0xff744f63
	PackVersion      = 2
)

type PackedObjectType byte

const (
	COMMIT              PackedObjectType = 1
	TREE                PackedObjectType = 2
	BLOB                PackedObjectType = 3
	TAG                 PackedObjectType = 4
	OBJECT_OFFSET_DELTA PackedObjectType = 6
	OBJECT_REF_DELTA    PackedObjectType = 7
)

type packedObject struct {
	Object
	bytes *[]byte
}

type Pack struct {
	// GIT currently accepts version number 2 or 3 but
	// generates version 2 only.
	version int32
	// the unpacked objects
	content []*packedObject
	*Idx
}

type Idx struct {
	// the object ids sorted by offset
	entries []*PackedObjectId
	// the object ids mapped by oid
	entriesById map[string]*PackedObjectId
	// number of objects contained in the pack (network
	// byte order)
	count int64
	// copy of the checksum for this idx file's corresponding pack file.
	packChecksum *ObjectId
	// checksum for this idx file.
	idxChecksum *ObjectId
}

type PackedObjectId struct {
	ObjectId
	offset int64
	crc32  int64
}

// ================================================================= //
// GGIT PACK PARSER
// ================================================================= //

type packIdxParser struct {
	idxParser  objectIdParser
	packParser dataParser
	name       string
}

// ================================================================= //
// Sorting of PackedObjectIds by offset.
// ================================================================= //

type packedObjectIds []*PackedObjectId

func (e packedObjectIds) Less(i, j int) bool {
	s := []*PackedObjectId(e)
	a, b := s[i], s[j]
	return a.offset < b.offset
}

func (e packedObjectIds) Swap(i, j int) {
	s := []*PackedObjectId(e)
	s[i], s[j] = s[j], s[i]
}

func (e packedObjectIds) Len() int {
	s := []*PackedObjectId(e)
	return len(s)
}

// ================================================================= //
// .idx parsing.
// ================================================================= //

func (p *packIdxParser) parseIdx() *Idx {
	p.idxParser.ConsumeString(PackIdxSignature)
	p.idxParser.ConsumeBytes([]byte{0, 0, 0, PackVersion})
	var counts [256]int64
	for i := range counts {
		counts[i] = p.idxParser.ParseIntBigEndian(4)
	}
	//discard the fan-out values, just use the largest value, which is the total # of objects:
	count := counts[255]
	entries := make([]*PackedObjectId, count, count)
	entriesByOid := make(map[string]*PackedObjectId)
	for i := int64(0); i < count; i++ {
		b := p.idxParser.ReadNBytes(20)
		representation := fmt.Sprintf("%x", b)
		entries[i] = &PackedObjectId{
			ObjectId: ObjectId{
				b,
				representation,
			},
		}
		entriesByOid[representation] = entries[i]
	}
	for i := int64(0); i < count; i++ {
		entries[i].crc32 = int64(p.idxParser.ParseIntBigEndian(4))
	}
	for i := int64(0); i < count; i++ {
		//TODO: 8-byte #'s for some offsets for some pack files (packs > 2gb)
		entries[i].offset = p.idxParser.ParseIntBigEndian(4)
	}
	checksumPack := p.idxParser.ReadNBytes(20)
	packChecksum := &ObjectId{
		bytes: checksumPack,
		repr:  fmt.Sprintf("%x", checksumPack),
	}
	//TODO: check the checksum
	checksumIdx := p.idxParser.ReadNBytes(20)
	idxChecksum := &ObjectId{
		bytes: checksumIdx,
		repr:  fmt.Sprintf("%x", checksumIdx),
	}
	if !p.idxParser.EOF() {
		panicErrf("Found extraneous bytes! %x", p.idxParser.Bytes())
	}
	//order by offset
	sort.Sort(packedObjectIds(entries))
	return &Idx{
		entries,
		entriesByOid,
		count,
		packChecksum,
		idxChecksum,
	}
}

// ================================================================= //
// .pack and pack entry parsing.
// ================================================================= //

type packedObjectParser struct {
	*objectParser
	bytes *[]byte
}

func newPackedObjectParser(data *[]byte, oid *ObjectId) (p *packedObjectParser, e error) {
	compressedReader := bytes.NewReader(*data)
	var zr io.ReadCloser
	if zr, e = zlib.NewReader(compressedReader); e == nil {
		exploder := &dataParser{
			bufio.NewReader(zr),
			0,
		}
		exploded := exploder.Bytes()
		explodedReader := bufio.NewReader(bytes.NewReader(exploded))
		op := newObjectParser(explodedReader, oid)
		pop := packedObjectParser{
			op,
			&exploded,
		}
		p = &pop
	}
	return
}

func (p *packIdxParser) parsePack() *Pack {
	idx := p.parseIdx()
	objects := make([]*packedObject, idx.count)
	p.packParser.ConsumeString(PackSignature)
	p.packParser.ConsumeBytes([]byte{0, 0, 0, PackVersion})
	count := p.packParser.ParseIntBigEndian(4)
	if count != idx.count {
		panicErrf("Pack file count doesn't match idx file count for pack-%s!", p.name) //todo: don't panic.
	}
	entries := &idx.entries
	data := p.packParser.Bytes()
	for i := range *entries {
		objects[i] = parseEntry(&data, i, idx, &objects)
	}
	return &Pack{
		PackVersion,
		objects,
		idx,
	}
}

func parseEntry(packedData *[]byte, i int, idx *Idx, packedObjects *[]*packedObject) (object *packedObject) {
	entries := idx.entries
	objects := *packedObjects
	data := *packedData
	v := entries[i]
	absoluteCursor := int(v.offset) - 12 //pack signature + pack version + count = 12 bytes
	if len(objects) > i && objects[i] != nil {
		//sometimes (for ref delta objects) we jump ahead in the []byte
		return objects[i]
	}
	var (
		size int64
		err  error
	)
	// keep track of bytes read so that, in conjunction with the next entry's offset, we can know where the next
	// object in the pack begins.
	relativeCursor := 0
	headerHeader := data[absoluteCursor+relativeCursor]
	relativeCursor++
	typeBits := (headerHeader & 127) >> 4
	sizeBits := (headerHeader & 15)
	//collect remaining size bytes, if any.
	sizeBytes := fmt.Sprintf("%04b", sizeBits)
	for s := headerHeader; isSetMSB(s); {
		s = data[absoluteCursor+relativeCursor]
		relativeCursor++
		sizeBytes = fmt.Sprintf("%07b", s&127) + sizeBytes
	}
	if size, err = strconv.ParseInt(sizeBytes, 2, 64); err != nil {
		panicErrf("Err parsing size: %v. Could not determine size for %s", err, v.repr)
	}
	pot := PackedObjectType(typeBits)
	var bytes []byte
	if i+1 < len(entries) {
		n := int(entries[i+1].offset - v.offset)
		bytes = data[absoluteCursor+relativeCursor : absoluteCursor+n]
		relativeCursor = n
	} else {
		//todo: limited reading so we don't gOOM
		bytes = data[absoluteCursor+relativeCursor:]
		absoluteCursor = len(data)
	}
	absoluteCursor += relativeCursor
	var dp *packedObjectParser
	switch {
	case pot == BLOB || pot == COMMIT || pot == TREE || pot == TAG:
		object = parseNonDeltaEntry(&bytes, pot, &v.ObjectId, size)
	default:
		var (
			deltaDeflated packedDelta
			base          *packedObject
			offset        int64
		)
		switch pot {
		case OBJECT_REF_DELTA:
			var oid *ObjectId
			deltaDeflated, oid = readPackedRefDelta(&bytes)
			offset = idx.entriesById[oid.String()].offset
		case OBJECT_OFFSET_DELTA:
			if deltaDeflated, offset, err = readPackedOffsetDelta(&bytes); err != nil {
				panicErrf("Err parsing size: %v. Could not determine size for %s", err, v.repr)
			}
			offset = v.offset - offset
		}
		objectIndex := sort.Search(len(idx.entries), func(i int) bool {
			return idx.entries[i].offset >= int64(offset)
		})
		if idx.entries[objectIndex].offset != offset {
			panicErrf("Could not find object with offset %d (%d - %d). Closest match was %d.", offset,
				v.offset+offset, v.offset, idx.entries[i].offset)
		}
		if objects[objectIndex] == nil {
			objects[objectIndex] = parseEntry(packedData, objectIndex, idx, packedObjects)
			if objects[objectIndex] == nil {
				panicErrf("Ref deltas not yet implemented!")
			}
		}
		base = objects[objectIndex]
		bytes = *((*[]byte)(deltaDeflated))
		if dp, err = newPackedObjectParser(&bytes, &v.ObjectId); err != nil {
			panicErr(err.Error())
		}
		object = dp.parseDelta(base, &v.ObjectId)
	}
	return
}

func parseNonDeltaEntry(bytes *[]byte, pot PackedObjectType, oid *ObjectId, size int64) (object *packedObject) {
	var (
		dp  *packedObjectParser
		err error
	)
	if dp, err = newPackedObjectParser(bytes, oid); err != nil {
		panicErr(err.Error())
	}
	switch pot {
	case BLOB:
		object = dp.parseBlob(size)
	case COMMIT:
		object = dp.parseCommit(size)
	case TREE:
		object = dp.parseTree(size)
	case TAG:
		object = dp.parseTag(size)
	}
	return
}

func (dp *packedObjectParser) parseCommit(size int64) *packedObject {
	dp.hdr = &objectHeader{
		ObjectCommit,
		int(size),
	}
	commit := dp.objectParser.parseCommit()

	return &packedObject{
		commit,
		dp.bytes,
	}
}
func (dp *packedObjectParser) parseTag(size int64) *packedObject {
	dp.hdr = &objectHeader{
		ObjectTag,
		int(size),
	}
	tag := dp.objectParser.parseTag()
	return &packedObject{
		tag,
		dp.bytes,
	}
}

func (dp *packedObjectParser) parseBlob(size int64) *packedObject {
	blob := new(Blob)
	blob.data = dp.Bytes()
	blob.oid = dp.objectParser.oid
	blob.hdr = &objectHeader{
		ObjectBlob,
		int(size),
	}
	return &packedObject{
		blob,
		&blob.data,
	}
}

func (dp *packedObjectParser) parseTree(size int64) *packedObject {
	dp.hdr = &objectHeader{
		ObjectTree,
		int(size),
	}
	tree := dp.objectParser.parseTree()
	return &packedObject{
		tree,
		dp.bytes,
	}
}

// ================================================================= //
// Delta parsing.
// ================================================================= //

type packedDelta *[]byte

func readPackedRefDelta(bytes *[]byte) (delta packedDelta, oid *ObjectId) {
	baseOidBytes := (*bytes)[0:20]
	deltaBytes := (*bytes)[20:]
	delta = packedDelta(&deltaBytes)
	oid, _ = OidFromBytes(baseOidBytes)
	return packedDelta(&deltaBytes), oid
}

func readPackedOffsetDelta(bytes *[]byte) (delta packedDelta, offset int64, err error) {
	//first the offset to the base object earlier in the pack
	var i int
	offset, err, i = parseOffset(bytes)
	//now the rest of the bytes - the compressed delta
	deltaBytes := (*bytes)[i:]
	delta = packedDelta(&deltaBytes)
	return
}

func parseOffset(bytes *[]byte) (offset int64, err error, index int) {
	offsetBits := ""
	var base int64
	for i := 0; ; {
		v := (*bytes)[i]
		offsetBits = offsetBits + fmt.Sprintf("%07b", v&127)
		if i >= 1 {
			base += int64(1 << (7 * uint(i)))
		}
		if !isSetMSB(v) {
			if offset, err = strconv.ParseInt(offsetBits, 2, 64); err != nil {
				return
			}
			offset += base
			index = i + 1
			break
		}
		i++
	}
	return
}

func (p *objectParser) readByteAsInt() int64 {
	return int64(p.ReadByte())
}

func (dp *packedObjectParser) parseDelta(base *packedObject, id *ObjectId) (object *packedObject) {
	return nil
}

// Compute an integer value in the format that pack files use for a delta's base
// size and output size. The function is named after the decoding mechanism:
// bytes are read and computed until a byte is found whose most significant
// bit is not set.
func parseIntWhileMSB(readByte func() (int, byte, bool)) (i int64) {
	n := 0
	for {
		_, v, _ := readByte()
		i |= (int64(v&127) << (uint(n) * 7))
		if !isSetMSB(v) {
			break
		}
		n++
	}
	return i
}

// ================================================================= //
// UTIL METHODS
// ================================================================= //

//return true if the most significant bit is set, false otherwise
func isSetMSB(b byte) bool {
	return b > 127
}
