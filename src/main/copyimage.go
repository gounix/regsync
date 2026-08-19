package main

import (
	"encoding/json"
	"github.com/gounix/goregistry/v2"
	"github.com/gounix/gosecret"
	"log/slog"
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
		err = copyBlob(srcReg, dstReg,  layer.MediaType, layer.Digest)
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
	err = dstReg.PutManifest(mediaType, ret.Raw, digest)
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

func copyFilteredManifestList(srcReg goregistry.RegistryT, srcCred gosecret.RegCredT, dstReg goregistry.RegistryT, dstCred gosecret.RegCredT, tag string) error {
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

		if entry.Platform.Architecture == "amd64" {

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

func copyImage(srcReg goregistry.RegistryT, dstReg goregistry.RegistryT, tag string) error {
	var err error

	err = srcReg.AcquireToken()
        if err != nil {
                slog.Error("main.copyImage get src token", "err", err)
		return err
        }

	err = dstReg.AcquirePushToken()
        if err != nil {
                slog.Error("main.copyImage get src token", "err", err)
		return err
        }

	//err = copyFilteredManifestList(srcReg, srcReg.Regcred, dstReg, dstReg.Regcred, tag)
	err = copyManifestList(srcReg, srcReg.Regcred, dstReg, dstReg.Regcred, tag)
	if err != nil {
		err = copyManifest(srcReg, dstReg, "application/vnd.docker.distribution.manifest.v2+json,application/vnd.oci.image.manifest.v1+json", tag)
		if err != nil {
			slog.Error("main.copyImage copyManifest", "err", err)
			return err
		}
	}
	return nil
}
