//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
//
package api

import (
	"bufio"
	"compress/zlib"
	"errors"
	"fmt"
	"github.com/jbrukh/ggit/api/objects"
	"github.com/jbrukh/ggit/api/parse"
	"github.com/jbrukh/ggit/util"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// a representation of a git repository
type DiskRepository struct {
	path       string
	packs      []*parse.Pack
	packedRefs []objects.Ref
}

// Open a reprository that is located at the given path. The
// path of a repository is always its .git directory. However,
// if the enclosing directory is given, then ggit will
// append the .git directory to the specified path.
// TODO: really should be using the same logic here as in ggit,
// which finds the closest top-level repo.
func Open(pth string) *DiskRepository {
	p := util.InferGitDir(pth)
	return &DiskRepository{
		path: p,
	}
}

// Destroy is a highly destructive operation that 
// irrevocably destroys the git repository and its
// enclosing directory.
func (repo *DiskRepository) Destroy() error {
	dir, _ := filepath.Split(repo.path)
	return os.RemoveAll(dir)
}

func (repo *DiskRepository) ObjectFromOid(oid *objects.ObjectId) (obj objects.Object, err error) {
	var (
		f  *os.File
		e  error
		rz io.ReadCloser
	)
	if f, e = objectFile(repo, oid); e != nil {
		if os.IsNotExist(e) {
			if err := loadPacks(repo); err != nil {
				return nil, err
			}
			if obj, ok := parse.Unpack(repo.packs, oid); ok {
				return obj, nil
			}
		}
		return nil, e
	}
	defer f.Close() // just in case
	if rz, e = zlib.NewReader(f); e != nil {
		return nil, e
	}
	defer rz.Close()
	file := bufio.NewReader(rz)
	p := parse.NewObjectParser(file, oid)
	return p.ParsePayload()
}

func (repo *DiskRepository) ObjectFromShortOid(short string) (objects.Object, error) {
	l := len(short)
	if l < 4 || l > objects.OidHexSize {
		return nil, fmt.Errorf("fatal: Not a valid object name %s", short)
	}

	// don't bother with directories if we know the full SHA
	if l == objects.OidHexSize {
		oid, err := objects.OidFromString(short)
		if err != nil {
			return nil, fmt.Errorf("fatal: Not a valid object name %s", short)
		}
		return repo.ObjectFromOid(oid)
	}

	head, tail := short[:2], short[2:]
	root := path.Join(repo.path, DefaultObjectsDir, head)
	var matching []*objects.ObjectId
	e := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// root doesn't exist, or there was a problem reading it
		if err != nil {
			return err
		}
		if !info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, tail) {
				if oid, err := objects.OidFromString(head + name); err == nil {
					matching = append(matching, oid)
				}
			}
		}
		return nil
	})
	if e != nil {
		if os.IsNotExist(e) {
			loadPacks(repo)
			if obj, ok := parse.UnpackFromShortOid(repo.packs, short); ok {
				return obj, nil
			}
		}
		return nil, e
	}
	if len(matching) != 1 {
		return nil, fmt.Errorf("fatal: Ambiguous object name %s", short)
	}
	return repo.ObjectFromOid(matching[0])
}

// Ref is a repository-based baseline method for getting refs. The
// ref spec is the full path of the ref that is relative to the .git
// directory.
//
// This will attempt to open the file pointed to by the spec. If this
// file is unavailable, packed refs are loaded into memory (and cached)
// and it attempts to find the ref there.
//
// For smart disambiguation of refs, or ref peeling, thou shalt
// use helper operations.
func (repo *DiskRepository) Ref(spec string) (objects.Ref, error) {
	file, e := relativeFile(repo, spec)
	if e == nil {
		defer file.Close()
		p := parse.NewRefParser(bufio.NewReader(file), spec)
		return p.ParseRef()
	}
	if os.IsNotExist(e) {
		refs, err := repo.PackedRefs()
		if err != nil {
			return nil, noSuchRefErrf(spec)
		}
		for _, r := range refs {
			if r.Name() == spec {
				return r, nil
			}
		}
		return nil, noSuchRefErrf(spec)
	}
	return nil, e
}

func (repo *DiskRepository) PackedObjectIds() ([]*objects.ObjectId, error) {
	if err := loadPacks(repo); err != nil {
		return nil, err
	}
	return parse.ObjectIdsFromPacks(repo.packs), nil
}

func (repo *DiskRepository) PackedObjects() ([]*parse.PackedObject, error) {
	if err := loadPacks(repo); err != nil {
		return nil, err
	}
	return parse.ObjectsFromPacks(repo.packs), nil
}

func (repo *DiskRepository) LooseObjectIds() (oids []*objects.ObjectId, err error) {
	objectsRoot := path.Join(repo.path, DefaultObjectsDir)
	oids = make([]*objects.ObjectId, 0)
	//look in each objectsDir and make ObjectIds out of the files there.
	err = filepath.Walk(objectsRoot, func(path string, info os.FileInfo, errr error) error {
		if name := info.Name(); name == "info" || name == "pack" {
			return filepath.SkipDir
		} else if !info.IsDir() {
			hash := filepath.Base(filepath.Dir(path)) + name
			var oid *objects.ObjectId
			if oid, err = objects.OidFromString(hash); err != nil {
				return err
			}
			oids = append(oids, oid)
		}
		return nil
	})
	return
}

func (repo *DiskRepository) Index() (idx *Index, err error) {
	file, e := relativeFile(repo, IndexFile)
	if e != nil {
		return nil, e
	}
	defer file.Close()
	return toIndex(bufio.NewReader(file))
}

func (repo *DiskRepository) PackedRefs() ([]objects.Ref, error) {
	if repo.packedRefs == nil {
		file, e := relativeFile(repo, PackedRefsFile)
		if e != nil {
			return nil, e
		}
		defer file.Close()
		p := parse.NewRefParser(bufio.NewReader(file), "")
		var refs []objects.Ref
		if refs, e = p.ParsePackedRefs(); e != nil {
			return nil, e
		}
		repo.packedRefs = refs
	}
	return repo.packedRefs, nil
}

func (repo *DiskRepository) LooseRefs() ([]objects.Ref, error) {
	// TODO: figure out a way to decouple this logic
	repoPath := repo.path + "/"
	dir := path.Join(repoPath, "refs")
	refs := make([]objects.Ref, 0)
	err := filepath.Walk(dir,
		func(path string, f os.FileInfo, err error) error {
			if !f.IsDir() {
				name := util.TrimPrefix(path, repoPath)
				r, e := PeeledRefFromSpec(repo, name)
				if e != nil {
					return e
				}
				refs = append(refs, objects.NewRef(name, "", r.ObjectId(), nil))
			}
			return nil
		},
	)
	return refs, err
}

func (repo *DiskRepository) Refs() ([]objects.Ref, error) {

	// First, get all the packed refs.
	pr, err := repo.PackedRefs()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Refs will be stores in a map by their symbolic name.
	refs := make(map[string]objects.Ref)
	for _, ref := range pr {
		refs[ref.Name()] = ref
	}

	// Now let's walk loose refs and collect them to supercede
	// the packed refs. It is worth it to note here that
	// packed refs may contain outdated references because
	// they are updated lazily.
	dir := path.Join(repo.path, "refs")
	err = filepath.Walk(dir,
		func(path string, f os.FileInfo, err error) error {
			// refs are files, so...
			if !f.IsDir() {
				name := util.TrimPrefix(path, repo.path+"/")
				r, e := PeeledRefFromSpec(repo, name)
				if e != nil {
					return e
				}
				refs[name] = objects.NewRef(name, "", r.ObjectId(), nil)
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	// collect the refs into a list
	refList := make([]objects.Ref, 0, len(refs))
	for _, v := range refs {
		refList = append(refList, v)
	}
	sort.Sort(refByName(refList))
	return refList, nil
}

// ================================================================= //
// OPERATIONS
// ================================================================= //

// List all objects in a disk repository.
func ObjectIds(repo *DiskRepository) (oids []*objects.ObjectId, err error) {
	pOids, err := repo.PackedObjectIds()
	if err != nil {
		return nil, err
	}
	lOids, err := repo.LooseObjectIds()
	if err != nil {
		return nil, err
	}
	return append(pOids, lOids...), nil
}

// ================================================================= //
// UTILITY METHODS
// ================================================================= //

// AssertDiskRepo returns the DiskRepository if this is a DiskRepository,
// and an error otherwise.
func AssertDiskRepo(repo Repository) (*DiskRepository, error) {
	switch r := repo.(type) {
	case *DiskRepository:
		return r, nil
	}
	return nil, errors.New("fatal: not a disk repository")
}

// objectFile turns an oid into a path relative to the
// git directory of a repository where that object should
// be located (if it is a loose object).
func objectFile(repo *DiskRepository, oid *objects.ObjectId) (file *os.File, err error) {
	hex := oid.String()
	path := path.Join(repo.path, DefaultObjectsDir, hex[0:2], hex[2:])
	return os.Open(path)
}

// relativeFile returns the full path (including the repository path)
// of a path that is given relative to the .git directory of a 
// repository
func relativeFile(repo *DiskRepository, relPath string) (file *os.File, err error) {
	path := path.Join(repo.path, relPath)
	return os.Open(path)
}

// loadsPacks loads the packs of a repository.
func loadPacks(repo *DiskRepository) (err error) {
	if repo.packs != nil {
		return
	}
	objectsRoot := path.Join(repo.path, DefaultObjectsDir)
	packRoot := path.Join(objectsRoot, DefaultPackDir)
	packNames := make([]string, 0)
	if err = filepath.Walk(packRoot, func(path string, info os.FileInfo, ignored error) error {
		if strings.HasSuffix(path, "idx") {
			name := info.Name()
			packNames = append(packNames, parse.PackName(name))
		}
		return nil
	}); err != nil {
		return
	}
	packs := make([]*parse.Pack, len(packNames), len(packNames))
	for i, name := range packNames {
		if idxFile, e := os.Open(path.Join(packRoot, "pack-"+name+".idx")); e != nil {
			return e
		} else {
			defer idxFile.Close()
			packFile := "pack-" + name + ".pack"
			open := func() (*os.File, error) {
				return os.Open(path.Join(packRoot, packFile))
			}
			pp := parse.NewPackIdxParser(bufio.NewReader(idxFile), parse.Opener(open), name)
			packs[i] = pp.ParsePack()
		}
	}
	repo.packs = packs
	return
}
