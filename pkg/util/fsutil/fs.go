package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"syscall"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
)

// fsnode is one path of a merged tree: its position in the tree is its (renamed)
// path. Files point at their content via fsindex/originPath; dirs are served
// from the tree itself.
type fsnode struct {
	name       string
	children   []fsnode // lexically sorted by name
	fsindex    int      // index into FS.fses owning this file; emptyIndex for dirs
	originPath string   // the file's path within fses[fsindex]; unset for dirs
}

const emptyIndex = -1

// fs.PathError operation names of FS methods.
const (
	opMkdir   = "mkdir"
	opOpen    = "open"
	opRead    = "read"
	opReaddir = "readdir"
	opRename  = "rename"
	opStat    = "stat"
	opSub     = "sub"
)

// isDir reports whether n is a directory: dirs are tree-served and carry no origin.
func (n *fsnode) isDir() bool { return n.originPath == "" }

// FS is an ordered set of fs.FS merged into one virtual tree; on colliding
// paths, later merges own the path, like map merges. Rename moves paths within
// the tree; files keep their originPath, so content serves from its owner.
type FS struct {
	fses []fs.FS
	tree fsnode
}

// NewFS seeds the tree from fsys
func NewFS(fsys fs.FS) (*FS, error) {
	f := &FS{tree: fsnode{fsindex: emptyIndex}}
	if err := f.Merge(fsys); err != nil {
		return nil, err
	}
	return f, nil
}

// MustNewFS is NewFS, panicking on error.
func MustNewFS(fsys fs.FS) *FS {
	f, err := NewFS(fsys)
	if err != nil {
		panic(err)
	}
	return f
}

// Merge merges fsys into the tree; it takes ownership of any path it holds.
func (f *FS) Merge(fsys fs.FS) error {
	f.fses = append(f.fses, fsys)
	return f.mergeDir(&f.tree, fsys, len(f.fses)-1, "")
}

// mergeDir lays dir's entries under node, recursing into subdirectories via
// fs.Sub so the call stack walks the tree; prefix is dir's path in fsys.
func (f *FS) mergeDir(node *fsnode, dir fs.FS, index int, prefix string) error {
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := node.mkdirAll(entry.Name())
		if entry.IsDir() {
			sub, err := fs.Sub(dir, entry.Name())
			if err != nil {
				return err
			}
			if err := f.mergeDir(child, sub, index, prefix+entry.Name()+"/"); err != nil {
				return err
			}
			continue
		}
		child.fsindex, child.originPath = index, prefix+entry.Name()
	}
	return nil
}

// MustMerge is Merge, panicking on error, returning the receiver for chaining.
func (f *FS) MustMerge(fsys fs.FS) *FS {
	if err := f.Merge(fsys); err != nil {
		panic(err)
	}
	return f
}

// MkdirAll creates tree-only directories along name; a file at name or along
// the way fails with not-a-directory, naming that file.
func (f *FS) MkdirAll(path string) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: opMkdir, Path: path, Err: fs.ErrInvalid}
	}
	return f.ensureDir(opMkdir, path, true)
}

// Rename moves oldPath's subtree to newPath; each file keeps its originPath, so
// content serves from its owner wherever it lands. Missing dirs along newPath
// are created; an existing same-kind node at newPath is replaced, a different
// kind fails like rename(2).
func (f *FS) Rename(oldPath, newPath string) error {
	switch {
	case !fs.ValidPath(newPath): // !fs.ValidPath(oldPath) runs in f.lookup() just below
		return fmt.Errorf("fsutil: invalid rename destination %q", newPath)
	case oldPath == "." || newPath == ".":
		return fmt.Errorf("fsutil: cannot rename %q", ".")
	case newPath == oldPath:
		return nil
	case strings.HasPrefix(newPath, oldPath+"/"):
		return &fs.PathError{Op: opRename, Path: newPath, Err: syscall.EINVAL} // a dir into its own subtree
	}
	node, parent, err := f.lookup(opRename, oldPath)
	if err != nil {
		return fmt.Errorf("fsutil: rename source %q not found", oldPath)
	}
	if err := f.ensureDir(opRename, path.Dir(newPath), false); err != nil {
		return err
	}
	if err := f.checkRenameDest(node, newPath); err != nil {
		return err
	}

	moved := *node // value copy: detaching shifts parent's children
	parent.children = slices.DeleteFunc(parent.children, func(n fsnode) bool { return n.name == moved.name })

	moved.name = path.Base(newPath)
	*f.tree.mkdirAll(newPath) = moved
	return nil
}

// ensureDir walks p from the tree root, failing on any file along it with
// not-a-directory naming that file. Missing dirs are created when create,
// skipped otherwise.
func (f *FS) ensureDir(op, p string, create bool) error {
	prefix, node := "", &f.tree
	for seg := range strings.SplitSeq(p, "/") {
		child, i := node.child(seg)
		if child != nil && !child.isDir() {
			return &fs.PathError{Op: op, Path: prefix + seg, Err: syscall.ENOTDIR}
		}
		if child == nil {
			if !create {
				return nil // nothing below exists yet
			}
			node.children = slices.Insert(node.children, i, fsnode{name: seg, fsindex: emptyIndex})
			child = &node.children[i]
		}
		prefix += seg + "/"
		node = child
	}
	return nil
}

// checkRenameDest rejects a destination rename(2) would refuse: a file over an
// existing dir (EISDIR), a dir over an existing file (ENOTDIR), or a dir over
// a non-empty dir (ENOTEMPTY).
func (f *FS) checkRenameDest(node *fsnode, newPath string) error {
	existing, _, err := f.lookup(opRename, newPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) { // unreachable: newPath is valid and not "."
		return err
	}
	switch {
	case existing == nil:
		return nil
	case !node.isDir() && existing.isDir():
		return &fs.PathError{Op: opRename, Path: newPath, Err: syscall.EISDIR}
	case node.isDir() && !existing.isDir():
		return &fs.PathError{Op: opRename, Path: newPath, Err: syscall.ENOTDIR}
	case len(existing.children) > 0:
		return &fs.PathError{Op: opRename, Path: newPath, Err: syscall.ENOTEMPTY}
	}
	return nil
}

// Open opens the file or directory at name; files serve content from their
// owner at originPath.
func (f *FS) Open(name string) (fs.File, error) {
	node, _, err := f.lookup(opOpen, name)
	if err != nil {
		return nil, err
	}
	if node.isDir() {
		return &fakeDir{fsys: f, node: node, path: name}, nil
	}
	file, err := f.fses[node.fsindex].Open(node.originPath)
	if err != nil {
		return nil, err
	}
	return namedFile{File: file, name: path.Base(name)}, nil
}

// Stat implements fs.StatFS.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	node, _, err := f.lookup(opStat, name)
	if err != nil {
		return nil, err
	}
	return f.statNode(node, name)
}

// ReadDir implements fs.ReadDirFS.
func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer log.Defer("readDir file close", file.Close)
	dir, ok := file.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: opReaddir, Path: name, Err: errors.New("not a directory")}
	}
	return dir.ReadDir(-1)
}

// Sub implements fs.SubFS: it returns the tree rooted at dir as its own FS,
// sharing content ownership with f.
func (f *FS) Sub(dir string) (fs.FS, error) {
	if dir == "." {
		return f, nil
	}
	node, _, err := f.lookup(opSub, dir)
	if err != nil {
		return nil, err
	}
	if !node.isDir() {
		return nil, &fs.PathError{Op: opSub, Path: dir, Err: errors.New("not a directory")}
	}
	return &FS{fses: f.fses, tree: *node}, nil
}

// lookup finds the node naming p and its parent (nil for "."), validating p
// per io/fs rules.
func (f *FS) lookup(op, p string) (*fsnode, *fsnode, error) {
	if p == "." {
		return &f.tree, nil, nil
	}
	if !fs.ValidPath(p) {
		return nil, nil, &fs.PathError{Op: op, Path: p, Err: fs.ErrInvalid}
	}
	return f.tree.lookup(op, p)
}

// lookup returns the node at p under n and its parent, failing when any
// segment of p is absent.
func (n *fsnode) lookup(op, p string) (*fsnode, *fsnode, error) {
	return n.walkPath(p, func(cur *fsnode, name string) (*fsnode, error) {
		c, _ := cur.child(name)
		if c == nil {
			return nil, &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist}
		}
		return c, nil
	})
}

// mkdirAll returns the node at slash-separated path p under n, creating
// tree-only dirs along the way.
func (n *fsnode) mkdirAll(p string) *fsnode {
	node, _, _ := n.walkPath(p, func(cur *fsnode, name string) (*fsnode, error) {
		c, i := cur.child(name)
		if c == nil {
			cur.children = slices.Insert(cur.children, i, fsnode{name: name, fsindex: emptyIndex})
			c = &cur.children[i]
		}
		return c, nil
	})
	return node
}

// walkPath walks slash-separated p from n, one segment at a time: step
// resolves each segment's child under the current node, or errors to stop the
// walk. It returns the node at p and the node holding it.
func (n *fsnode) walkPath(p string, step func(cur *fsnode, name string) (*fsnode, error)) (*fsnode, *fsnode, error) {
	node, parent := n, (*fsnode)(nil)
	for seg := range strings.SplitSeq(p, "/") {
		child, err := step(node, seg)
		if err != nil {
			return nil, nil, err
		}
		parent, node = node, child
	}
	return node, parent, nil
}

// statNode types n at its tree path p: directories from the tree itself,
// files from their owner's entry at originPath.
func (f *FS) statNode(n *fsnode, p string) (fs.FileInfo, error) {
	name := path.Base(p)
	if n.isDir() {
		return dirInfo{name: name}, nil
	}
	info, err := fs.Stat(f.fses[n.fsindex], n.originPath)
	if err != nil {
		return nil, err
	}
	return fileInfo{FileInfo: info, name: name}, nil
}

// child returns n's child named name and its index among n's children; the
// child is nil when absent, the index its would-be position.
func (n *fsnode) child(name string) (*fsnode, int) {
	i, found := slices.BinarySearchFunc(n.children, name, func(c fsnode, target string) int {
		return strings.Compare(c.name, target)
	})
	if found {
		return &n.children[i], i
	}
	return nil, i
}
