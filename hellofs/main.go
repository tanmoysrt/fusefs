package main

import (
	"context"
	"log"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/hellofs"
	"github.com/jacobsa/timeutil"
)

func main() {
	server, err := hellofs.NewHelloFS(timeutil.RealClock())
	if err != nil {
		log.Fatalf("failed to create hellofs: %v", err)
	}

	// Try to unmount if it's already mounted.
	_ = fuse.Unmount("/mnt/test")

	// Mount the file system.
	mfs, err := fuse.Mount("/mnt/test", server, &fuse.MountConfig{
		ReadOnly: false,
		FSName:   "hellofs",
	})
	if err != nil {
		log.Fatalf("failed to mount: %v", err)
	}

	// Wait for it to be unmounted.
	if err = mfs.Join(context.Background()); err != nil {
		log.Fatalf("Join: %v", err)
	}
}
