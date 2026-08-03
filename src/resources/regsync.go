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
	"log/slog"
	"github.com/gounix/gok8s"
)

const (
	rsKind        = "regsyncs"
)

type (
	SrcT struct {
		RegistryName string `json:"registryName"`
		Image        string `json:"image"`
		Tag          string `json:"tag"`
	}
	TargetT struct {
		RegistryName string `json:"registryName"`
		Image        string `json:"image"`
	}
	RsSpecT struct {
		Src    SrcT    `json:"src"`
		Target TargetT `json:"target"`
	}
	RegsyncT struct {
		Metadata MetadataT `json:"metadata"`
		Spec     RsSpecT   `json:"spec"`
	}
	RegsyncListT struct {
		ApiVersion string     `json:"apiVersion"`
		Items      []RegsyncT `json:"items"`
	}
)

func (dat RegsyncListT) dumpRegsyncs() {
	slog.Info("resources.dumpRegsyncs", "ApiVersion", dat.ApiVersion)
	for _, entry := range dat.Items {
		slog.Info("resources.dumpRegsyncs", "Metadata.Name", entry.Metadata.Name, "Metadata.Namespace", entry.Metadata.Namespace)
		slog.Info("resources.dumpRegsyncs", 
		    "Spec.Src", fmt.Sprintf("%s/%s:%s", entry.Spec.Src.RegistryName, entry.Spec.Src.Image, entry.Spec.Src.Tag), 
		    "Spec.Target", fmt.Sprintf("%s/%s", entry.Spec.Target.RegistryName, entry.Spec.Target.Image))
	}
}

func GetRegsyncList() (RegsyncListT, error) {
	// get the list of regsync resources from k8s
	var dat RegsyncListT

	url := fmt.Sprintf("/apis/%s/%s/%s/", api, api_version, rsKind)
	out, err := gok8s.GetClientSet().RESTClient().Get().AbsPath(url).DoRaw(context.TODO())
	if err != nil {
		slog.Error("resources.GetRegsyncList", "clientset.RESTClient", err)
		return dat, err
	}

	err = json.Unmarshal(out, &dat)
	if err != nil {
		slog.Error("resources.GetRegsyncList", "unmarshal error", err)
		return dat, err
	}

	//dat.dumpRegsyncs()
	slog.Info("resources.GetRegsyncList", "entries", len(dat.Items))
	return dat, nil
}
