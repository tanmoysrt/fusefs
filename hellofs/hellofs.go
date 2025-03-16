package main

import (
	"context"
	"io"
	"strings"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/timeutil"
)

func NewHelloFS(clock timeutil.Clock) (fuse.Server, error) {
	fs := &HelloFS{
		Clock: clock,
	}

	return fuseutil.NewFileSystemServer(fs), nil
}

func (fs *HelloFS) StatFS(ctx context.Context, op *fuseops.StatFSOp) error {
	return nil
}

func (fs *HelloFS) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	parent, ok := StaticInodeInfo[op.Parent]
	if !ok {
		return fuse.ENOENT
	}

	// Find the child with the given name.
	var foundChild *fuseutil.Dirent
	for _, child := range parent.Children {
		if child.Name == op.Name {
			foundChild = &child
			break
		}
	}
	if foundChild == nil {
		return fuse.ENOENT
	}

	// Update the inode ID.
	op.Entry.Child = foundChild.Inode
	op.Entry.Attributes = StaticInodeInfo[foundChild.Inode].Attributes
	fs.SetDefaultAttributes(&op.Entry.Attributes)
	return nil
}

func (fs *HelloFS) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	info, ok := StaticInodeInfo[op.Inode]
	if !ok {
		return fuse.ENOENT
	}

	op.Attributes = info.Attributes
	fs.SetDefaultAttributes(&op.Attributes)
	return nil
}

func (fs *HelloFS) OpenDir(ctx context.Context, op *fuseops.OpenDirOp) error {
	return nil
}

func (fs *HelloFS) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	info, ok := StaticInodeInfo[op.Inode]
	if !ok {
		return fuse.ENOENT
	}

	if !info.IsDir {
		return fuse.ENOTDIR
	}

	// If requested offset is beyond the end of the directory, return io.EOF.
	if op.Offset > fuseops.DirOffset(len(info.Children)) {
		return nil
	}

	entries := info.Children[op.Offset:]

	//
	for _, entry := range entries {
		i := fuseutil.WriteDirent(op.Dst[op.BytesRead:], entry)
		if i == 0 {
			// 0 means that the buffer was too small and cannot write dirent
			break
		}
		op.BytesRead += i
	}

	return nil
}

func (fs *HelloFS) OpenFile(ctx context.Context, op *fuseops.OpenFileOp) error {
	return nil
}

func (fs *HelloFS) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	// Let io.ReaderAt deal with the semantics.
	reader := strings.NewReader("Hello, world!")

	var err error
	op.BytesRead, err = reader.ReadAt(op.Dst, op.Offset)

	// Special case: FUSE doesn't expect us to return io.EOF.
	if err == io.EOF {
		return nil
	}

	return err
}

// Utility
func (fs *HelloFS) SetDefaultAttributes(attr *fuseops.InodeAttributes) {
	now := fs.Clock.Now()
	attr.Atime = now
	attr.Mtime = now
	attr.Crtime = now
}
