package main

import (
	"context"
	"log"
	"os"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/timeutil"
)

func main() {
	server, err := NewHelloFS(timeutil.RealClock())
	if err != nil {
		log.Fatalf("failed to create hellofs: %v", err)
	}

	// Try to unmount if it's already mounted.
	_ = fuse.Unmount("/mnt/test")

	// Mount the file system.
	cfg := fuse.MountConfig{
		ReadOnly: false,
		FSName:   "hellofs",
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
