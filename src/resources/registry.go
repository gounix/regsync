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

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"errors"
	"log/slog"
	"github.com/gounix/gok8s"
	"regsync/environ"
)

const (
	api         = "gounix.nl"
	api_version = "v1"
	kind        = "registries"
)

type (
	MetadataT struct {
		Uid       string `json:"uid"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	}
	SpecT struct {
		Scheme               string `json:"scheme"`
		TlsVerify            bool   `json:"tlsVerify"`
		Host                 string `json:"host"`
		SupportChunkedUpload bool   `json:"supportChunkedUpload"`
		Authenticated        bool   `json:"authenticated"`
		SecretName           string `json:"secretName"`
	}
	RegistryT struct {
		Metadata MetadataT `json:"metadata"`
		Spec     SpecT     `json:"spec"`
	}
	RegistryListT struct {
		ApiVersion string      `json:"apiVersion"`
		Items      []RegistryT `json:"items"`
	}
)

func dumpRegistries(dat RegistryListT) {
	slog.Info("resources.dumpRegistries", "ApiVersion", dat.ApiVersion)
	for _, entry := range dat.Items {
		slog.Info("resources.dumpRegistries", "Name", entry.Metadata.Name, "Namespace", entry.Metadata.Namespace)
		slog.Info("resources.dumpRegistries", 
			"Scheme", entry.Spec.Scheme, 
			"tlsVerify", entry.Spec.TlsVerify, 
			"Host", entry.Spec.Host, 
			"SupportChunkedUpload", entry.Spec.SupportChunkedUpload, 
			"Authenticated", entry.Spec.Authenticated, 
			"SecretName", entry.Spec.SecretName)
	}
}

func GetRegistryList() (RegistryListT, error) {
	// get the list of registry resources from k8s
	var dat RegistryListT

	url := fmt.Sprintf("/apis/%s/%s/%s/", api, api_version, kind)
	out, err := gok8s.GetClientSet().RESTClient().Get().AbsPath(url).DoRaw(context.TODO())
	if err != nil {
		slog.Error("resources.GetRegistryList", "clientset.RESTClient", err)
		return dat, err
	}

	err = json.Unmarshal(out, &dat)
	if err != nil {
		slog.Error("resources.GetRegistryList", "unmarshal error", err)
		return dat, err
	}

	//dumpRegistries(dat)
	slog.Info("resources.GetRegistryList", "entries", len(dat.Items))
	return dat, nil
}

func (dat RegistryListT) GetRegistry(name string, namespace string) (RegistryT, error) {
	// first search in specified namespace
	for _, entry := range dat.Items {
		if entry.Metadata.Name == name && entry.Metadata.Namespace == namespace {
			slog.Info("resources.GetRegistry", "registry", fmt.Sprintf("%s/%s", namespace, name))
			return entry, nil
		}
	}
	// if not found search in regsync namespace
	for _, entry := range dat.Items {
		if entry.Metadata.Name == name && entry.Metadata.Namespace == environ.Env.RegsyncNamespace {
			slog.Info("resources.GetRegistry", "registry", fmt.Sprintf("%s/%s", environ.Env.RegsyncNamespace, name))
			return entry, nil
		}
	}
	slog.Error("resources.GetRegistry not found", "registry", fmt.Sprintf("%s/%s", namespace, name))
	return RegistryT{}, errors.New("registry not found")
}
