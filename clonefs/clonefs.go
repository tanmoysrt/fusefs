package main

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/timeutil"
)

/*
Clone Filesystem

- It just stores files in clone folder
- No directory support (only files) [just for simplicity of inode management]
- No sym links (so no CreateLink/Unlink)
- No extra attributes (so no GetXattr/ListXattr/SetXattr/RemoveXattr)
- No fallocate
- File UID/GID set to 1000
- All files are 0777 (rwx)
*/

type CloneFS struct {
	fuseutil.NotImplementedFileSystem

	Clock       timeutil.Clock
	CloneFolder string

	InodeMap       map[fuseops.InodeID]Inode // inode ID -> inode
	FileInodeIdMap map[string]fuseops.InodeID
	HandleIDMap    map[fuseops.HandleID]fuseops.InodeID
	Dirents        []fuseutil.Dirent

	NextInodeID fuseops.InodeID
}

type Inode struct {
	Name       string
	Size       uint64
	IsDir      bool
	References uint64
}

func NewCloneFS(clock timeutil.Clock, cloneFolder string) (fuse.Server, error) {
	fs := &CloneFS{
		Clock:       clock,
		CloneFolder: cloneFolder,
		InodeMap: map[fuseops.InodeID]Inode{
			fuseops.RootInodeID: {
				Name:       ".",
				Size:       0,
				IsDir:      true,
				References: 0,
			},
		},
		FileInodeIdMap: map[string]fuseops.InodeID{
			".": fuseops.RootInodeID,
		},
		HandleIDMap: map[fuseops.HandleID]fuseops.InodeID{},
		NextInodeID: fuseops.RootInodeID + 1,
		Dirents:     []fuseutil.Dirent{},
	}

	// Create the clone folder if it doesn't exist
	if _, err := os.Stat(cloneFolder); os.IsNotExist(err) {
		err = os.Mkdir(cloneFolder, 0755)
		if err != nil {
			return nil, err
		}
	}

	return fuseutil.NewFileSystemServer(fs), nil
}

// Inode Functions

func (fs *CloneFS) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	if op.Parent != fuseops.RootInodeID {
		return nil
	}
	inodeID, ok := fs.FileInodeIdMap[op.Name]
	if !ok {
		return fuse.ENOENT
	}
	info, ok := fs.InodeMap[inodeID]
	if !ok {
		return fuse.ENOENT
	}
	info.References++
	op.Entry = fuseops.ChildInodeEntry{
		Child: inodeID,
	}
	SetDefaultAttributes(&op.Entry.Attributes)
	if info.IsDir {
		op.Entry.Attributes.Mode = os.ModeDir | 0777
	}
	if !info.IsDir {
		// re-fetch size from file system
		op.Entry.Attributes.Size = info.Size
		stat, err := os.Stat(fs.CloneFolder + "/" + info.Name)
		if err == nil {
			op.Entry.Attributes.Size = uint64(stat.Size())
		}
	}
	// TODO set dentry exception
	return nil
}

func (fs *CloneFS) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	info, ok := fs.InodeMap[op.Inode]
	if !ok {
		return fuse.ENOENT
	}
	SetDefaultAttributes(&op.Attributes)
	if info.IsDir {
		op.Attributes.Mode = os.ModeDir | 0777
	}
	if !info.IsDir {
		// re-fetch size from file system
		op.Attributes.Size = info.Size
		stat, err := os.Stat(fs.CloneFolder + "/" + info.Name)
		if err == nil {
			op.Attributes.Size = uint64(stat.Size())
		}
	}
	return nil
}

func (fs *CloneFS) SetInodeAttributes(ctx context.Context, op *fuseops.SetInodeAttributesOp) error {
	// Ignore for now
	return nil
}

func (fs *CloneFS) ForgetInode(ctx context.Context, op *fuseops.ForgetInodeOp) error {
	fs.forgetInode(op.Inode, op.N)
	return nil
}

func (fs *CloneFS) BatchForget(ctx context.Context, op *fuseops.BatchForgetOp) error {
	for _, entry := range op.Entries {
		fs.forgetInode(entry.Inode, entry.N)
	}
	return nil
}

func (fs *CloneFS) MkNode(ctx context.Context, op *fuseops.MkNodeOp) error {
	// Not implemented
	return fuse.ENOSYS
}

// Directory Related
func (fs *CloneFS) OpenDir(ctx context.Context, op *fuseops.OpenDirOp) error {
	if op.Inode == fuseops.RootInodeID {
		op.CacheDir = false
		op.KeepCache = false
		fs.HandleIDMap[op.Handle] = fuseops.RootInodeID
		return nil
	}
	return fuse.ENOENT
}

func (fs *CloneFS) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	if op.Inode != fuseops.RootInodeID {
		return nil
	}

	fs.CreateDirents()
	if op.Offset > fuseops.DirOffset(len(fs.Dirents)) {
		return nil
	}

	entries := fs.Dirents[op.Offset:]

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

func (fs *CloneFS) ReleaseDirHandle(ctx context.Context, op *fuseops.ReleaseDirHandleOp) error {
	delete(fs.HandleIDMap, op.Handle)
	return nil
}

// File Management

func (fs *CloneFS) CreateFile(ctx context.Context, op *fuseops.CreateFileOp) error {
	if op.Parent != fuseops.RootInodeID {
		return fuse.ENOSYS
	}
	// check if the file already exists
	_, err := os.Stat(fs.CloneFolder + "/" + op.Name)
	if err == nil {
		return fuse.EEXIST
	}
	// create the file in clone folder
	file, err := os.Create(fs.CloneFolder + "/" + op.Name)
	if err != nil {
		return err
	}
	file.Close()

	// Create the inode
	inodeID := fs.NextInodeID
	fs.NextInodeID++
	fs.FileInodeIdMap[op.Name] = inodeID
	fs.InodeMap[inodeID] = Inode{
		Name:       op.Name,
		Size:       0,
		IsDir:      false,
		References: 1,
	}

	fs.HandleIDMap[op.Handle] = inodeID

	op.Entry = fuseops.ChildInodeEntry{
		Child: inodeID,
	}
	return nil
}

func (fs *CloneFS) OpenFile(ctx context.Context, op *fuseops.OpenFileOp) error { return nil }

func (fs *CloneFS) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	inode, ok := fs.InodeMap[op.Inode]
	if !ok {
		return fuse.ENOENT
	}
	file, err := os.Open(fs.CloneFolder + "/" + inode.Name) // TODO We can cache this fd with HandleID to prevent open/close
	if err != nil {
		return err
	}
	defer file.Close()
	offset := int64(op.Offset)
	size := int64(op.Size)

	data := make([]byte, size)
	n, err := file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return err
	}

	copy(op.Dst, data)
	op.BytesRead = n
	return nil
}

func (fs *CloneFS) WriteFile(ctx context.Context, op *fuseops.WriteFileOp) error {
	inode, ok := fs.InodeMap[op.Inode]
	if !ok {
		return fuse.ENOENT
	}
	file, err := os.OpenFile(fs.CloneFolder+"/"+inode.Name, os.O_WRONLY, 0777)
	if err != nil {
		return err
	}
	defer file.Close()
	offset := int64(op.Offset)
	_, err = file.WriteAt(op.Data, offset)
	if err != nil {
		return err
	}
	return nil
}

func (fs *CloneFS) Rename(ctx context.Context, op *fuseops.RenameOp) error {
	// can be called for a file or a directory
	// Reject directory renames (since we don't support directories here)
	return fuse.ENOSYS
}

func (fs *CloneFS) SyncFile(ctx context.Context, op *fuseops.SyncFileOp) error { return nil }

func (fs *CloneFS) FlushFile(ctx context.Context, op *fuseops.FlushFileOp) error {
	// Here we should just commit file and close it
	return nil
}

func (fs *CloneFS) ReleaseFileHandle(ctx context.Context, op *fuseops.ReleaseFileHandleOp) error {
	delete(fs.HandleIDMap, op.Handle)
	return nil
}

// File System Functions
func (fs *CloneFS) StatFS(ctx context.Context, op *fuseops.StatFSOp) error {
	// OS X specific
	return nil
}

func (fs *CloneFS) SyncFS(ctx context.Context, op *fuseops.SyncFSOp) error { return nil }

func (fs *CloneFS) Unlink(ctx context.Context, op *fuseops.UnlinkOp) error {
	if op.Parent != fuseops.RootInodeID {
		return fuse.ENOSYS
	}

	inodeID, ok := fs.FileInodeIdMap[op.Name]
	if !ok {
		return fuse.ENOENT
	}
	inode, ok := fs.InodeMap[inodeID]
	if !ok {
		return fuse.ENOENT
	}
	delete(fs.InodeMap, inodeID)
	delete(fs.FileInodeIdMap, op.Name)
	// Remove the file from the folder
	err := os.Remove(fs.CloneFolder + "/" + inode.Name)
	if err != nil {
		return err
	}
	return nil
}

// Utility Functions
func SetDefaultAttributes(attr *fuseops.InodeAttributes) {
	attr.Nlink = 1
	attr.Mode = 0777
	attr.Uid = 1000
	attr.Gid = 1000
	attr.Atime = time.Now()
	attr.Ctime = time.Now()
}

func (fs *CloneFS) forgetInode(inodeID fuseops.InodeID, N uint64) {
	if inode, ok := fs.InodeMap[inodeID]; ok {
		inode.References -= N
		if inode.References <= 0 {
			delete(fs.InodeMap, inodeID)
			delete(fs.FileInodeIdMap, inode.Name)
		}
	}
}

func (fs *CloneFS) CreateDirents() {
	// find the files in clone directory
	files, err := os.ReadDir(fs.CloneFolder)
	if err != nil {
		return
	}
	// create the inodes of files if not exist
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if _, ok := fs.FileInodeIdMap[file.Name()]; !ok {
			inodeID := fs.NextInodeID
			fs.NextInodeID++
			fs.FileInodeIdMap[file.Name()] = inodeID
			var size uint64 = 0
			info, err := file.Info()
			if err == nil {
				size = uint64(info.Size())
			}
			fs.InodeMap[inodeID] = Inode{
				Name:       file.Name(),
				Size:       size,
				IsDir:      false,
				References: 0,
			}
		}
	}
	entries := make([]fuseutil.Dirent, len(files))
	for i, file := range files {
		if file.IsDir() {
			continue
		}
		inodeID := fs.FileInodeIdMap[file.Name()]
		entries[i] = fuseutil.Dirent{
			Offset: fuseops.DirOffset(i + 1),
			Inode:  inodeID,
			Name:   file.Name(),
			Type:   fuseutil.DT_File,
		}
	}
	fs.Dirents = entries
}

// ##################################### Unsupported ops ####################################
// - No directory support (only files) [just for simplicity of inode management]
// - No sym links (so no CreateLink/Unlink)
// - No extra attributes (so no GetXattr/ListXattr/SetXattr/RemoveXattr)
// - No fallocate

// Directories
func (fs *CloneFS) MkDir(ctx context.Context, op *fuseops.MkDirOp) error { return fuse.ENOSYS }

func (fs *CloneFS) RmDir(ctx context.Context, op *fuseops.RmDirOp) error { return fuse.ENOSYS }

// Links

func (fs *CloneFS) CreateLink(ctx context.Context, op *fuseops.CreateLinkOp) error {
	return fuse.ENOSYS
}

func (fs *CloneFS) CreateSymlink(ctx context.Context, op *fuseops.CreateSymlinkOp) error {
	return fuse.ENOSYS
}

func (fs *CloneFS) ReadSymlink(ctx context.Context, op *fuseops.ReadSymlinkOp) error {
	return fuse.ENOSYS
}

// Extra attributes

func (fs *CloneFS) GetXattr(ctx context.Context, op *fuseops.GetXattrOp) error { return nil }

func (fs *CloneFS) ListXattr(ctx context.Context, op *fuseops.ListXattrOp) error { return nil }

func (fs *CloneFS) RemoveXattr(ctx context.Context, op *fuseops.RemoveXattrOp) error {
	return fuse.ENOSYS
}

func (fs *CloneFS) SetXattr(ctx context.Context, op *fuseops.SetXattrOp) error { return fuse.ENOSYS }

// Fallocate
func (fs *CloneFS) Fallocate(ctx context.Context, op *fuseops.FallocateOp) error { return fuse.ENOSYS }
