package main

import (
	"github.com/gounix/goregistry/v2"
	"log/slog"
)

func copyBlob(src goregistry.RegistryT, dst goregistry.RegistryT, mediaType string, digest string) error {

	// check if it exists
	code, err := dst.CheckBlob(mediaType, digest)
	if err != nil {
		slog.Error("main.copyBlob CheckBlob", "err", err)
		return err
	}
	slog.Info("main.copyBlob", "code", code)
	if code == 200 {
		slog.Info("main.copyBlob blob already present")
		return nil
	}

	// read blob from source
	blob, err := src.GetBlob(mediaType, digest)
	if err != nil {
		slog.Error("main.copyBlob GetBlob", "err", err)
		return err
	}

	// get upload location from dst
	location, err := dst.PostBlob(mediaType, digest)
	if err != nil {
		slog.Error("main.copyBlob PostBlob", "err", err)
		return err
	}

	// upload blob to location
	location, err = dst.PatchBlob(location, mediaType, digest, blob)
	if err != nil {
		// cancel upload
		dst.DelBlob(location, mediaType, digest)
		slog.Error("main.copyBlob PatchBlob", "err", err)
		return err
	}

	// finish upload
	err = dst.PutBlob(location, mediaType, digest, blob)
	if err != nil {
		slog.Error("main.copyBlob PutBlob", "err", err)
		return err
	}

	return nil
}
