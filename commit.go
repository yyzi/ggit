package ggit

import (
    "bufio"
    "fmt"
    "io"
)

const (
    markerTree      = "tree"
    markerParent    = "parent"
    markerAuthor    = "author"
    markerCommitter = "committer"
)

type Commit struct {
    author    *PersonTimestamp
    committer *PersonTimestamp
    message   string
    tree      *ObjectId
    parents   []*ObjectId
    repo      Repository
}

func (c *Commit) Type() ObjectType {
    return ObjectCommit
}

func (c *Commit) String() string {
    const FMT = "Commit{author=%s, committer=%s, tree=%s, parents=%v, message='%s'}"
    return fmt.Sprintf(FMT, c.author, c.committer, c.tree, c.parents, c.message)
}

func (c *Commit) WriteTo(w io.Writer) (n int, err error) {
    return io.WriteString(w, c.String())
}

func (c *Commit) addParent(oid *ObjectId) {
    if c.parents == nil {
        c.parents = make([]*ObjectId, 0, 2)
    }
    c.parents = append(c.parents, oid)
}

func parseCommit(repo Repository, h *objectHeader, buf *bufio.Reader) (c *Commit, err error) {
    c = new(Commit)
    p := &dataParser{buf}
    err = dataParse(func() {

        // read the tree line
        p.ConsumeString(markerTree)
        p.ConsumeByte(SP)
        c.tree = p.ParseObjectId()
        p.ConsumeByte(LF)

        // read an arbitrary number of parent lines
        n := len(markerParent)
        for p.PeekString(n) == markerParent {
            p.ConsumeString(markerParent)
            p.ConsumeByte(SP)
            c.addParent(p.ParseObjectId())
            p.ConsumeByte(LF)
        }

        // read the author
        p.ConsumeString(markerAuthor)
        p.ConsumeByte(SP)
        line := p.ReadString(LF)                  // gets rid of the LF!
        c.author = &PersonTimestamp{line, "", ""} // TODO

        // read the committer
        p.ConsumeString(markerCommitter)
        p.ConsumeByte(SP)
        line = p.ReadString(LF)                      // gets rid of the LF!
        c.committer = &PersonTimestamp{line, "", ""} // TODO

        // read the commit message
        p.ConsumeByte(LF)
        c.message = p.String()
    })
    return
}
