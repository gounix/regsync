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

package environ

import (
	"go-simpler.org/env"
	"os"
	"log/slog"
)

type EnvT struct {
	Standalone       bool   `env:"STANDALONE" default:false`
	GarbageCollector bool   `env:"GARBAGE_COLLECTOR" default:false`
	//SyncerImage      string `env:"SYNCER_IMAGE,required"`
	//SyncerRepo       string `env:"SYNCER_REPO,required"`
	//SyncerTag        string `env:"SYNCER_TAG,required"`
	//SyncerPullPolicy string `env:"SYNCER_PULLPOLICY" default:"Always"`
	//SyncerCPU        string `env:"SYNCER_CPU" default:"100m"`
	//SyncerMEM        string `env:"SYNCER_MEM" default:"256Mi"`
	Interval         string `env:"INTERVAL" default:"24h"`
	//SyncerNamespace  string `env:"SYNCER_NAMESPACE,required"`
	RegsyncNamespace string `env:"REGSYNC_NAMESPACE,required"`
	Port             int    `env:"PORT,required"`
}

var Env EnvT

func Load() error {
	if err := env.Load(&Env, nil); err != nil {
		slog.Error("regsync/environ", "env.Load", err)
		os.Exit(1)
	}
	slog.Info("regsync.environ loaded environment", 
		"STANDALONE", Env.Standalone, 
		"GARBAGE_COLLECTOR", Env.GarbageCollector,
		//"SYNCER_IMAGE", Env.SyncerImage, 
		//"SYNCER_REPO", Env.SyncerRepo, 
		//"SYNCER_TAG", Env.SyncerTag, 
		//"SYNCER_PULLPOLICY", Env.SyncerPullPolicy, 
		//"SYNCER_CPU", Env.SyncerCPU,
		//"SYNCER_MEM", Env.SyncerMEM,
		"INTERVAL", Env.Interval,
		//"SYNCER_NAMESPACE", Env.SyncerNamespace,
		"REGSYNC_NAMESPACE", Env.RegsyncNamespace,
		"PORT", Env.Port)
	return nil
}
