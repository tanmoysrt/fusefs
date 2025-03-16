package main

import (
	"os"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/timeutil"
)

type HelloFS struct {
	fuseutil.NotImplementedFileSystem

	Clock timeutil.Clock
}

const (
	RootInode  fuseops.InodeID = fuseops.RootInodeID
	HelloInode fuseops.InodeID = fuseops.RootInodeID + 1
	DirInode   fuseops.InodeID = fuseops.RootInodeID + 2
	WorldInode fuseops.InodeID = fuseops.RootInodeID + 3
)

type InodeInfo struct {
	Children   []fuseutil.Dirent
	Attributes fuseops.InodeAttributes
	IsDir      bool
}

var StaticInodeInfo = map[fuseops.InodeID]InodeInfo{
	// root
	RootInode: InodeInfo{
		Attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0555 | os.ModeDir,
		},
		IsDir: true,
		Children: []fuseutil.Dirent{
			fuseutil.Dirent{
				Offset: 1,
				Inode:  HelloInode,
				Name:   "hello",
				Type:   fuseutil.DT_File,
			},
			fuseutil.Dirent{
				Offset: 2,
				Inode:  DirInode,
				Name:   "dir",
				Type:   fuseutil.DT_Directory,
			},
		},
	},
	HelloInode: InodeInfo{
		Attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
			Size:  uint64(len("Hello, world!")),
		},
	},
	DirInode: InodeInfo{
		Attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0555 | os.ModeDir,
		},
		IsDir: true,
		Children: []fuseutil.Dirent{
			fuseutil.Dirent{
				Offset: 1,
				Inode:  WorldInode,
				Name:   "world",
				Type:   fuseutil.DT_File,
			},
		},
	},
	WorldInode: InodeInfo{
		Attributes: fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0444,
			Size:  uint64(len("Hello, world!")),
		},
	},
}
