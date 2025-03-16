package main

import (
	"context"
	"log"
	"os"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/timeutil"
)

/*
Simple Clone Filesystem

On the host /home/tanmoy/volumes is a directory which will hold all the volumes.

We can mount /home/tanmoy/volumes/test to /mnt/test

So that all the files actually stored in /home/tanmoy/volumes/test and the mount is just temporary
*/

func main() {
	server, err := NewCloneFS(timeutil.RealClock(), "/home/tanmoy/Desktop/volumes/test1")
	if err != nil {
		log.Fatalf("failed to create hellofs: %v", err)
	}

	// Try to unmount if it's already mounted.
	_ = fuse.Unmount("/mnt/test")

	// Mount the file system.
	cfg := fuse.MountConfig{
		ReadOnly: false,
		FSName:   "clonefs",
	}
	cfg.DebugLogger = log.New(os.Stderr, "fuse: ", 0)

	mfs, err := fuse.Mount("/mnt/test", server, &cfg)
	if err != nil {
		log.Fatalf("failed to mount: %v", err)
	}

	// Wait for it to be unmounted.
	if err = mfs.Join(context.Background()); err != nil {
		log.Fatalf("Join: %v", err)
	}
}
