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
	"encoding/json"
	"github.com/gounix/goregistry/v2"
	"github.com/gounix/gosecret"
	"log/slog"
	"regsync/resources"
)

func copyManifest(srcReg goregistry.RegistryT, dstReg goregistry.RegistryT, mediaType string, digest string) error {


	ret, err := srcReg.GetManifest(mediaType, digest)
	if err != nil {
		slog.Error("main.copyManifest GetManifest", "err", err)
		return err
	}

	// first send all the layers
	slog.Info("main.copyManifest putlayers", "total", len(ret.Json.Layers))
	for num, layer := range ret.Json.Layers {
		slog.Info("main.copyManifest putlayers", "number", num, "digest", layer.Digest, "size", layer.Size)
		if dstReg.SupportChunkedUpload {
			slog.Info("main.copyManifest using chunked upload")
			err = streamCopyBlob(srcReg, dstReg,  layer.MediaType, layer.Digest)
		} else {
			slog.Info("main.copyManifest using bulk upload")
			err = copyBlob(srcReg, dstReg,  layer.MediaType, layer.Digest)
		}
		if err != nil {
			slog.Error("main.copyManifest copyBlob layer", "err", err)
			return err
		}

	}

	// then send the config blob
	err = copyBlob(srcReg, dstReg, ret.Json.Config.MediaType, ret.Json.Config.Digest)
	if err != nil {
		slog.Error("main.copyManifest copyBlob config", "err", err)
		return err
	}

	// finally send the manifest
	err = dstReg.PutManifest(ret.Json.MediaType, ret.Raw, digest)
	if err != nil {
		slog.Error("main.copyManifest PutManifest", "err", err)
		return err
	}
	return nil
}

func copyManifestList(srcReg goregistry.RegistryT, srcCred gosecret.RegCredT, dstReg goregistry.RegistryT, dstCred gosecret.RegCredT, tag string) error {

	ret, err := srcReg.GetManifestList(tag)
        if err != nil {
                slog.Error("main.copyManifestList GetManifestList", "err", err)
		return err
        }

	for _, entry := range ret.Json.Manifest {

		slog.Info("main.copyManifestList", "arch", entry.Platform.Architecture, "os", entry.Platform.Os)
		slog.Info("main.copyManifestList", "MediaType", entry.MediaType, "Digest", entry.Digest)

		//if entry.Platform.Architecture == "amd64" && entry.Platform.Os == "linux" {
		// als er gefiltered wordt, dan geen of een aangepaste multi arch manifest wegschrijven
		err = copyManifest(srcReg, dstReg, entry.MediaType, entry.Digest)
		if err != nil {
			slog.Error("main.copyManifestList copyManifest", "err", err)
			return err
		}
	}

	err = dstReg.PutManifest(ret.Json.MediaType, ret.Raw, tag)
	if err != nil {
		slog.Error("main.copyManifestList PutManifest", "err", err)
		return err
	}
	return nil
}

func inFilter(architecture string, os string, filter []resources.FilterT) bool {
	if len(filter) == 0 {
		// empty filter matches all
		slog.Info("main.inFilter empty filter matches")
		return true
	}
	for _, entry := range filter {
		if entry.Architecture == architecture && entry.Os == os {
			slog.Info("main.inFilter match", "architecture", architecture, "os", os)
			return true
		}
	}
	slog.Error("main.inFilter no match")
	return false
}

func copyFilteredManifestList(srcReg goregistry.RegistryT, srcCred gosecret.RegCredT, dstReg goregistry.RegistryT, dstCred gosecret.RegCredT, filter []resources.FilterT, tag string) error {
	var new goregistry.ManifestListT

	ret, err := srcReg.GetManifestList(tag)
        if err != nil {
                slog.Error("main.copyManifestList GetManifestList", "err", err)
		return err
        }
	new.Json.SchemaVersion = ret.Json.SchemaVersion
	new.Json.MediaType = ret.Json.MediaType
	new.Json.Manifest = []goregistry.ArchManifestT{}

	for _, entry := range ret.Json.Manifest {

		slog.Info("main.copyFilteredManifestList", "arch", entry.Platform.Architecture, "os", entry.Platform.Os)
		slog.Info("main.copyFilteredManifestList", "MediaType", entry.MediaType, "Digest", entry.Digest)

		if inFilter(entry.Platform.Architecture, entry.Platform.Os, filter) {

			err = copyManifest(srcReg, dstReg, entry.MediaType, entry.Digest)
			if err != nil {
				slog.Error("main.copyFilteredManifestList copyManifest", "arch", entry.Platform.Architecture, "err", err)
				return err
			}
			new.Json.Manifest = append(new.Json.Manifest, entry)
		}
	}

	new.Raw, err = json.Marshal(new.Json)
	if err != nil {
		slog.Error("main.copyFilteredManifestList json.Marshal", "err", err)
		return err
	}

	err = dstReg.PutManifest(new.Json.MediaType, new.Raw, tag)
	if err != nil {
		slog.Error("main.copyFilteredManifestList PutManifest", "err", err)
		return err
	}
	return nil
}

func copyImage(srcReg goregistry.RegistryT, dstReg goregistry.RegistryT, filter []resources.FilterT, tag string) error {
	var err error

	// check if the source is a manifestlist or a manifest
	_, err = srcReg.GetManifestList(tag)
	if err == nil {
		err = copyFilteredManifestList(srcReg, srcReg.Regcred, dstReg, dstReg.Regcred, filter, tag)
		if err != nil {
			slog.Error("main.copyImage copyManifestList", "err", err)
			return err
		}
	} else {
		err = copyManifest(srcReg, dstReg, "application/vnd.docker.distribution.manifest.v2+json,application/vnd.oci.image.manifest.v1+json", tag)
		if err != nil {
			slog.Error("main.copyImage copyManifest", "err", err)
			return err
		}
	}
	return nil
}
