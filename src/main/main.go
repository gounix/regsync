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
	"github.com/gounix/goregistry"
	"github.com/gounix/gosecret"
	"log/slog"
	"regsync/environ"
	"regsync/jobs"
	"regsync/resources"
	"strings"
	"time"
)

func dumpRegistries(registries resources.RegistryListT) {
	for _, entry := range registries.Items {
                slog.Info("main.dumpRegistries",
			"Name", entry.Metadata.Name, "Namespace", entry.Metadata.Namespace,
                        "Scheme", entry.Spec.Scheme, "tlsVerify", entry.Spec.TlsVerify,
                        "Host", entry.Spec.Host, "Authenticated", entry.Spec.Authenticated, "SecretName", entry.Spec.SecretName)
		gometricsvr.PutLine("regsync_registries", 1.0, map[string]string{
			"name": entry.Metadata.Name, 
			"namespace": entry.Metadata.Namespace,
			"scheme": entry.Spec.Scheme,
			"tlsverify": fmt.Sprintf("%t", entry.Spec.TlsVerify),
			"host": entry.Spec.Host, 
			"authenticated": fmt.Sprintf("%t", entry.Spec.Authenticated), 
			"secretname": entry.Spec.SecretName,
		})
        }

}

func dumpRegSyncs(regsyncs resources.RegsyncListT) {
        for _, entry := range regsyncs.Items {
                slog.Info("resources.dumpRegsyncs",
			"Metadata.Name", entry.Metadata.Name, "Metadata.Namespace", entry.Metadata.Namespace,
			"Spec.Src", fmt.Sprintf("%s/%s:%s", entry.Spec.Src.RegistryName, entry.Spec.Src.Image, entry.Spec.Src.Tag),
			"Spec.Target", fmt.Sprintf("%s/%s", entry.Spec.Target.RegistryName, entry.Spec.Target.Image))
		gometricsvr.PutLine("regsync_configs", 1.0, map[string]string{
			"name": entry.Metadata.Name, 
			"namespace": entry.Metadata.Namespace,
			"src": fmt.Sprintf("%s/%s:%s", entry.Spec.Src.RegistryName, entry.Spec.Src.Image, entry.Spec.Src.Tag),
			"target": fmt.Sprintf("%s/%s", entry.Spec.Target.RegistryName, entry.Spec.Target.Image),
		})
        }

}

func checkRegistry(registries resources.RegistryListT, registryName string, image string) (resources.RegistryT, gosecret.RegCredT, goregistry.TokenT, error) {
	// getregistry
	host, err := registries.GetRegistry(registryName, environ.Env.RegsyncNamespace)
	if err != nil {
		slog.Error("main.checkRegistry", "registry", registryName, "err", err)
		return resources.RegistryT{}, gosecret.RegCredT{}, goregistry.TokenT(""), err
	}
	// getcredentials
	cred, err := gosecret.GetCredentials(host.Spec.Authenticated, environ.Env.RegsyncNamespace, host.Spec.SecretName)
	if err != nil {
		slog.Error("main.checkRegistry", "secretName", host.Spec.SecretName, "err", err)
		return resources.RegistryT{}, gosecret.RegCredT{}, goregistry.TokenT(""), err
	}
	// acquiretoken
	token, err := goregistry.AcquireToken(host.Spec.Scheme, host.Spec.TlsVerify, host.Spec.Host, image, cred)
	if err != nil {
		slog.Error("main.checkRegistry get src token", "err", err)
		return resources.RegistryT{}, gosecret.RegCredT{}, goregistry.TokenT(""), err
	}
	return host, cred, token, nil
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
		var list []string

		slog.Info("main.Cycle processing entry", 
			"src", entry.Spec.Src.RegistryName, 
			"dst", entry.Spec.Target.RegistryName, 
			"tag pattern", entry.Spec.Src.Tag)

		srcHost, srcCred, srcToken, err := checkRegistry(registries, entry.Spec.Src.RegistryName, entry.Spec.Src.Image)
		if err != nil {
			slog.Error("main.Cycle get source registry", "err", err)
			gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
				"name": fmt.Sprintf("%s/%s", entry.Metadata.Namespace, entry.Metadata.Name),
				"base": fmt.Sprintf("%s/%s", srcHost.Spec.Host, entry.Spec.Src.Image),
				"target": "",
				"updated": "false",
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
				"err": strings.ReplaceAll(err.Error(),"\"", ""),
			})
			continue
		}
		dstHost, dstCred, dstToken, err := checkRegistry(registries, entry.Spec.Target.RegistryName, entry.Spec.Target.Image)
		if err != nil {
			slog.Error("main.Cycle get dest registry", "err", err)
			gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
				"name": fmt.Sprintf("%s/%s", entry.Metadata.Namespace, entry.Metadata.Name),
				"base": fmt.Sprintf("%s/%s", srcHost.Spec.Host, entry.Spec.Src.Image),
				"target": fmt.Sprintf("%s/%s", dstHost.Spec.Host, entry.Spec.Target.Image),
				"updated": "false",
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
				"err": strings.ReplaceAll(err.Error(),"\"", ""),
			})
			continue
		}

		list, err = srcToken.GetVersions(srcHost.Spec.Scheme, srcHost.Spec.TlsVerify, srcHost.Spec.Host, entry.Spec.Src.Image, entry.Spec.Src.Tag)
		if err != nil {
			slog.Error("main.Cycle get versions", "err", err)
			gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
				"name": fmt.Sprintf("%s/%s", entry.Metadata.Namespace, entry.Metadata.Name),
				"base": fmt.Sprintf("%s/%s", srcHost.Spec.Host, entry.Spec.Src.Image),
				"target": fmt.Sprintf("%s/%s", dstHost.Spec.Host, entry.Spec.Target.Image),
				"updated": "false",
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
				"err": strings.ReplaceAll(err.Error(),"\"", ""),
			})
			continue
		}

		for _, tag := range list {
			slog.Info("main processing", "tag", tag)
			srcTime, err := srcToken.GetLastUpdate(srcHost.Spec.Scheme, srcHost.Spec.TlsVerify, srcHost.Spec.Host, entry.Spec.Src.Image, tag)
			if err != nil {
				slog.Error("main.Cycle get source time", "err", err)
				gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
					"name": fmt.Sprintf("%s/%s", entry.Metadata.Namespace, entry.Metadata.Name),
					"base": fmt.Sprintf("%s/%s:%s", srcHost.Spec.Host, entry.Spec.Src.Image, tag),
					"target": fmt.Sprintf("%s/%s:%s", dstHost.Spec.Host, entry.Spec.Target.Image, tag),
					"updated": "false",
					"timestamp": time.Now().Format("2006-01-02 15:04:05"),
					"err": strings.ReplaceAll(err.Error(),"\"", ""),
				})
				continue
			}
			slog.Info("main.Cycle", "tag", tag, "srcTime", srcTime)
			dstTime, _ := dstToken.GetLastUpdate(dstHost.Spec.Scheme, dstHost.Spec.TlsVerify, dstHost.Spec.Host, entry.Spec.Target.Image, tag)
			// in case of error it could be missing, so replicate it
			slog.Info("main.Cycle", "tag", tag, "dstTime", dstTime)
			updated := false
			message := ""
			if srcTime.After(dstTime) {
				slog.Info("main.Cycle", "tag", tag, "download", true)
				err := jobs.RunSyncJob(entry, tag, srcHost, dstHost, srcCred, dstCred)
				if err == nil {
					updated = true
				} else {
					message = err.Error()
				}
			}
			gometricsvr.PutLine("regsync_stats", 1.0, map[string]string{
				"name": fmt.Sprintf("%s/%s", entry.Metadata.Namespace, entry.Metadata.Name),
				"base": fmt.Sprintf("%s/%s:%s", srcHost.Spec.Host, entry.Spec.Src.Image, tag),
				"target": fmt.Sprintf("%s/%s:%s", dstHost.Spec.Host, entry.Spec.Target.Image, tag),
				"updated": fmt.Sprintf("%t", updated),
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
				"err": message,
			})
		}

	}
	slog.Info("main.Cycle finished")
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
		Cycle()
		time.Sleep(24 * time.Hour)
	}
}

