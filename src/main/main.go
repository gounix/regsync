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
	"fmt"
	"github.com/gounix/gok8s"
	"github.com/gounix/gometricsvr"
	"github.com/gounix/goregistry/v2"
	"github.com/gounix/gosecret"
	"log/slog"
	"regsync/environ"
	"regsync/resources"
	"strings"
	"time"
)

func dumpRegistries(registries resources.RegistryListT) {
	for _, entry := range registries.Items {
                slog.Info("main.dumpRegistries",
			"Name", entry.Metadata.Name, 
			"Namespace", entry.Metadata.Namespace,
                        "Scheme", entry.Spec.Scheme, 
			"tlsVerify", entry.Spec.TlsVerify,
                        "Host", entry.Spec.Host, 
			"SupportChunkedUpload", entry.Spec.SupportChunkedUpload, 
			"Authenticated", entry.Spec.Authenticated, 
			"SecretName", entry.Spec.SecretName)
		gometricsvr.PutLine("regsync_registries", 1.0, map[string]string{
			"name": entry.Metadata.Name, 
			"namespace": entry.Metadata.Namespace,
			"scheme": entry.Spec.Scheme,
			"tlsverify": fmt.Sprintf("%t", entry.Spec.TlsVerify),
			"host": entry.Spec.Host, 
			"SupportChunkedUpload": fmt.Sprintf("%t", entry.Spec.SupportChunkedUpload), 
			"authenticated": fmt.Sprintf("%t", entry.Spec.Authenticated), 
			"secretname": entry.Spec.SecretName,
		})
        }

}

func dumpFilter(filter []resources.FilterT) string {
	str := ""
        for _, entry := range filter {
		separator := ""
		if len(str) > 0 {
			separator = " or "
		}
		str = fmt.Sprintf("%s%s%s/%s", str, separator, entry.Architecture, entry.Os)
	}
	return str
}

func dumpRegSyncs(regsyncs resources.RegsyncListT) {
        for _, entry := range regsyncs.Items {
                slog.Info("resources.dumpRegsyncs",
			"Metadata.Name", entry.Metadata.Name, "Metadata.Namespace", entry.Metadata.Namespace,
			"Spec.Src", fmt.Sprintf("%s/%s:%s", entry.Spec.Src.RegistryName, entry.Spec.Src.Image, entry.Spec.Src.Tag),
			"Spec.Filter", dumpFilter(entry.Spec.Filter),
			"Spec.Target", fmt.Sprintf("%s/%s", entry.Spec.Target.RegistryName, entry.Spec.Target.Base))
		gometricsvr.PutLine("regsync_configs", 1.0, map[string]string{
			"name": entry.Metadata.Name, 
			"namespace": entry.Metadata.Namespace,
			"src": fmt.Sprintf("%s/%s:%s", entry.Spec.Src.RegistryName, entry.Spec.Src.Image, entry.Spec.Src.Tag),
			"target": fmt.Sprintf("%s/%s", entry.Spec.Target.RegistryName, entry.Spec.Target.Base),
		})
        }

}

func checkRegistry(registries resources.RegistryListT, registryName string, image string) (goregistry.RegistryT, error) {
	var reg goregistry.RegistryT

	// getregistry
	host, err := registries.GetRegistry(registryName, environ.Env.RegsyncNamespace)
	if err != nil {
		slog.Error("main.checkRegistry", "registry", registryName, "err", err)
		return goregistry.RegistryT{}, err
	}

	// getcredentials
	cred, err := gosecret.GetCredentials(host.Spec.Authenticated, environ.Env.RegsyncNamespace, host.Spec.SecretName)
	if err != nil {
		slog.Error("main.checkRegistry", "secretName", host.Spec.SecretName, "err", err)
		return goregistry.RegistryT{}, err
	}

	reg.Scheme = host.Spec.Scheme
        reg.TlsVerify = host.Spec.TlsVerify
        reg.Host = host.Spec.Host
        reg.SupportChunkedUpload = host.Spec.SupportChunkedUpload
        reg.Image = image
        reg.Regcred = cred

	// acquiretoken
	err = reg.AcquireToken()
	if err != nil {
		slog.Error("main.checkRegistry get token", "err", err)
		return goregistry.RegistryT{}, err
	}
	return reg, nil
}

func garbageCollect(reg goregistry.RegistryT, tag string) {

	if ! environ.Env.GarbageCollector {
		return
	}

	err := reg.AcquireDeleteToken()
	if err != nil {
		slog.Error("main.garbageCollect AcquireDeleteToken", "err", err)
		return
	}

	gcList, err := reg.GetVersions(tag, true)
	if err != nil {
		slog.Error("main.garbageCollect get versions for garbage collection", "err", err)
	}

	for _, gcTag := range gcList {
		slog.Info("main.garbageCollect", "image", reg.Image, "tag", gcTag)

		err = reg.DeleteImage(gcTag)
		if err != nil {
			slog.Error("main.garbageCollect delete image", "err", err)
		}

		gometricsvr.PutLine("regsync_gc", 1.0, map[string]string{
			"target": fmt.Sprintf("%s/%s:%s", reg.Host, reg.Image, gcTag),
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		})
	}
}

func ProcessImage(srcReg goregistry.RegistryT, dstReg goregistry.RegistryT, regsync resources.RegsyncT, tag string) error {

	err := dstReg.AcquirePushToken()
	if err != nil {
		slog.Error("main.ProcessImage AcquireToken", "err", err)
		gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
			"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
			"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, srcReg.Image, tag),
			"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, dstReg.Image, tag),
			"updated": "false",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"err": strings.ReplaceAll(err.Error(),"\"", ""),
		})
		return err
	}

	srcTime, err := srcReg.GetLastUpdate(tag)
	if err != nil {
		slog.Error("main.ProcessImage get source time", "err", err)
		gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
			"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
			"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, srcReg.Image, tag),
			"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, dstReg.Image, tag),
			"updated": "false",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"err": strings.ReplaceAll(err.Error(),"\"", ""),
		})
		return err
	}
	slog.Info("main.ProcessImage", "image", dstReg.Image, "tag", tag, "srcTime", srcTime)

	dstTime, err := dstReg.GetLastUpdate(tag)
	if err == nil && dstTime.IsZero() {
		// it exists, but we cannot determine the modified time, do not replicate
		slog.Error("main.ProcessImage cannot determine dst time but image exists, do not replicate")
		gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
			"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
			"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, srcReg.Image, tag),
			"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, dstReg.Image, tag),
			"updated": "false",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"err": "",
		})
		return nil
	}

	slog.Info("main.ProcessImage", "image", dstReg.Image, "tag", tag, "dstTime", dstTime, "err", err)
	updated := false
	message := ""
	// in case of error it could be missing, so replicate it
	if err != nil || srcTime.After(dstTime) {
		slog.Info("main.ProcessImage", "image", dstReg.Image, "tag", tag, "download", true)
		err = copyImage(srcReg, dstReg, regsync.Spec.Filter, tag)
		if err == nil {
			updated = true
		} else {
			message = err.Error()
		}
		gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
			"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
			"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, srcReg.Image, tag),
			"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, dstReg.Image, tag),
			"updated": fmt.Sprintf("%t", updated),
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"err": message,
		})
		return nil
	}
	slog.Info("main.ProcessImage", "image", dstReg.Image, "tag", tag, "download", false)
	gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
		"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
		"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, srcReg.Image, tag),
		"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, dstReg.Image, tag),
		"updated": "false",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"err": message,
	})
	return nil
}

func ProcessTag(srcReg goregistry.RegistryT, dstReg goregistry.RegistryT, regsync resources.RegsyncT) error {

	err := srcReg.AcquireToken()
	if err != nil {
		slog.Error("main.ProcessTag AcquireToken", "err", err)
		gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
			"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
			"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, srcReg.Image, regsync.Spec.Src.Tag),
			"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, dstReg.Image, regsync.Spec.Src.Tag),
			"updated": "false",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"err": strings.ReplaceAll(err.Error(),"\"", ""),
		})
		return err
	}

	list, err := srcReg.GetVersions(regsync.Spec.Src.Tag, false)
	if err != nil {
		slog.Error("main.ProcessTag GetVersions", "err", err)
		gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
			"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
			"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, srcReg.Image, regsync.Spec.Src.Tag),
			"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, dstReg.Image, regsync.Spec.Src.Tag),
			"updated": "false",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"err": strings.ReplaceAll(err.Error(),"\"", ""),
		})
		return err
	}

	garbageCollect(dstReg, regsync.Spec.Src.Tag)

	gometricsvr.PutLine("regsync_matches", 1.0, map[string]string{
		"base": fmt.Sprintf("%s/%s", srcReg.Host, srcReg.Image),
		"pattern": regsync.Spec.Src.Tag,
		"matches": fmt.Sprintf("%d", len(list)),
	})
	slog.Info("main.ProcessTag", "src image", srcReg.Image, "tags", list)
	for _, tag := range list {
		slog.Info("main.ProcessTag", "src image", srcReg.Image, "tag", tag)
		ProcessImage(srcReg, dstReg, regsync, tag)
	}
	return nil
}

func ProcessResource(srcReg goregistry.RegistryT, dstReg goregistry.RegistryT, regsync resources.RegsyncT) error {

	var imageList = []string{ regsync.Spec.Src.Image }
	var err error

	slog.Info("main.ProcessResource", "namespace", regsync.Metadata.Namespace, "name", regsync.Metadata.Name)
	if regsync.Spec.Src.ImageRegex != "" {
		srcReg.AcquireCatalogToken()
		imageList, err = srcReg.GetCatalog(regsync.Spec.Src.ImageRegex, false)
		if err != nil {
			slog.Error("main.ProcessResource GetCatalog", "err", err)
			gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
				"name": fmt.Sprintf("%s/%s", regsync.Metadata.Namespace, regsync.Metadata.Name),
				"base": fmt.Sprintf("%s/%s:%s", srcReg.Host, regsync.Spec.Src.ImageRegex, regsync.Spec.Src.Tag),
				"target": fmt.Sprintf("%s/%s:%s", dstReg.Host, regsync.Spec.Target.Base, regsync.Spec.Src.Tag),
				"updated": "false",
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
				"err": strings.ReplaceAll(err.Error(),"\"", ""),
			})
			return err
		}
	}
	slog.Info("main.ProcessResource", "matched", imageList)

	base := dstReg.Image
	for _, image := range imageList {
		slog.Info("main.ProcessResource", "processing", image)

		// fill in image name in src and dst
		srcReg.Image = image
		if base == "" {
			dstReg.Image = image
		} else {
			dstReg.Image = fmt.Sprintf("%s/%s", base, image)
		}

		ProcessTag(srcReg, dstReg, regsync)
	}
	return nil
}

func Cycle() {

	registries, err := resources.GetRegistryList()
	if err != nil || registries.Items == nil {
		slog.Error("main.Cycle no registries defined")
		return
	}
	dumpRegistries(registries)

	regsyncs, err := resources.GetRegsyncList()
	if err != nil || regsyncs.Items == nil {
		slog.Error("main.Cycle no regsyncs defined")
		return
	}
	dumpRegSyncs(regsyncs)

	for _, entry := range regsyncs.Items {

		slog.Info("main.Cycle processing entry", 
			"src", entry.Spec.Src.RegistryName, 
			"dst", entry.Spec.Target.RegistryName, 
			"imageRegex", entry.Spec.Src.ImageRegex,
			"image", entry.Spec.Src.Image,
			"filter", dumpFilter(entry.Spec.Filter),
			"tag pattern", entry.Spec.Src.Tag)

		srcReg, err := checkRegistry(registries, entry.Spec.Src.RegistryName, entry.Spec.Src.Image)
		if err != nil {
			slog.Error("main.Cycle get source registry", "err", err)
			gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
				"name": fmt.Sprintf("%s/%s", entry.Metadata.Namespace, entry.Metadata.Name),
				"base": fmt.Sprintf("%s/%s", srcReg.Host, entry.Spec.Src.Image),
				"target": "",
				"updated": "false",
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
				"err": strings.ReplaceAll(err.Error(),"\"", ""),
			})
			continue
		}
		dstReg, err := checkRegistry(registries, entry.Spec.Target.RegistryName, entry.Spec.Target.Base)
		if err != nil {
			slog.Error("main.Cycle get dest registry", "err", err)
			gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
				"name": fmt.Sprintf("%s/%s", entry.Metadata.Namespace, entry.Metadata.Name),
				"base": fmt.Sprintf("%s/%s", srcReg.Host, entry.Spec.Src.Image),
				"target": fmt.Sprintf("%s/%s", dstReg.Host, entry.Spec.Target.Base),
				"updated": "false",
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
				"err": strings.ReplaceAll(err.Error(),"\"", ""),
			})
			continue
		}
		ProcessResource(srcReg, dstReg, entry)

	}
	slog.Info("main.Cycle finished")
}

func waitForNext(start time.Time) {
	var num int64
	var unit string
	var interval time.Duration

	_, err := fmt.Sscanf(environ.Env.Interval, "%d%1s", &num, &unit)
	if err != nil {
		slog.Error("main.waitForNext", "INTERVAL", environ.Env.Interval, "err", err)
		num = 24
		unit = "h"
	}
	switch unit {
	case "h":
		interval = time.Duration(num) * time.Hour
	case "d":
		interval = time.Duration(24 * num) * time.Hour
	default:
		slog.Error("main.waitForNext", "illegal unit", unit)
		interval = time.Duration(24) * time.Hour
	}
	now := time.Now()
	active := now.Sub(start)
	todo := time.Duration(interval) - active
	slog.Info("main.waitForNext", "sleep", todo)
	time.Sleep(todo)
}

func main() {
	slog.Info("regsync.main run started", "version", "Development-version", "go", "Golang-version")
	if err := environ.Load(); err != nil {
                slog.Error("regsync.main", "environ.Load", err)
        }

	gometricsvr.PutHeader("regsync_version", "regsync version", "gauge")
	gometricsvr.PutKey("regsync_version", []string{"software"})
        gometricsvr.PutHeader("regsync_stats", "statistics of the regsync job", "gauge")
	gometricsvr.PutKey("regsync_stats", []string{"name", "base", "target"})
        gometricsvr.PutHeader("regsync_registries", "registries of the regsync job", "gauge")
	gometricsvr.PutKey("regsync_registries", []string{"name", "namespace"})
        gometricsvr.PutHeader("regsync_configs", "configs of the regsync job", "gauge")
	gometricsvr.PutKey("regsync_configs", []string{"name", "namespace"})
        gometricsvr.PutHeader("regsync_gc", "garbage collected images", "gauge")
	gometricsvr.PutKey("regsync_gc", []string{"target"})
        gometricsvr.PutHeader("regsync_matches", "number of tags matching pattern", "gauge")
	gometricsvr.PutKey("regsync_matches", []string{"base"})

	gometricsvr.PutLine("regsync_version", 1.0, map[string]string{
                "software": "regsync",
                "version": "Development-version",
        })
        gometricsvr.PutLine("regsync_version", 1.0, map[string]string{
                "software": "go",
                "version": "Golang-version",
        })
	// start web frontend and prometheus exporter
        gometricsvr.StartServer(environ.Env.Port)

	gok8s.InitConfig(environ.Env.Standalone)
	for {
		start := time.Now()
		Cycle()
		waitForNext(start)
	}
}

