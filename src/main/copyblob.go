/*
MIT License

Copyright (c) 2026 gounix

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"github.com/gounix/goregistry/v2"
	"log/slog"
)

const (
	mibibyte  = 1024 * 1024 // 1 MiB
	chunkSize = 10 * mibibyte
)

func copyBlob(src goregistry.RegistryT, dst goregistry.RegistryT, mediaType string, digest string) error {

	slog.Info("main.copyBlob", "digest", digest)

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
	location, err = dst.PatchBlob(location, mediaType, digest, blob.Raw)
	if err != nil {
		// cancel upload
		dst.DelBlob(location, mediaType, digest)
		slog.Error("main.copyBlob PatchBlob", "err", err)
		return err
	}

	// finish upload
	err = dst.PutBlob(location, mediaType, digest, blob.Raw)
	if err != nil {
		slog.Error("main.copyBlob PutBlob", "err", err)
		return err
	}

	slog.Info("main.copyBlob OK")
	return nil
}

func streamCopyBlob(src goregistry.RegistryT, dst goregistry.RegistryT, mediaType string, digest string) error {

	slog.Info("main.streamCopyBlob", "digest", digest)

	// check if it exists
	code, err := dst.CheckBlob(mediaType, digest)
	if err != nil {
		slog.Error("main.streamCopyBlob CheckBlob", "err", err)
		return err
	}
	slog.Info("main.streamCopyBlob", "code", code)
	if code == 200 {
		slog.Info("main.streamCopyBlob blob already present")
		return nil
	}

	// create a channel for transferring the blob
	ch := make(chan byte, chunkSize)

	// read blob from source
	go src.StreamReadBlob(mediaType, digest, ch, chunkSize)

	// get upload location from dst
	location, err := dst.PostBlob(mediaType, digest)
	if err != nil {
		slog.Error("main.streamCopyBlob PostBlob", "err", err)
		return err
	}

	// upload blob to location
	location, err = dst.StreamWriteBlob(location, mediaType, digest, ch, chunkSize)
	if err != nil {
		// cancel upload
		dst.DelBlob(location, mediaType, digest)
		slog.Error("main.streamCopyBlob PatchBlob", "err", err)
		return err
	}

	// finish upload
	err = dst.PutBlob(location, mediaType, digest, nil)
	if err != nil {
		slog.Error("main.streamCopyBlob PutBlob", "err", err)
		return err
	}

	slog.Info("main.streamCopyBlob OK")
	return nil
}
