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

package jobs

import (
	"context"
	"errors"
	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"regsync/environ"
	"log/slog"
	"regsync/resources"
	"github.com/gounix/gok8s"
	"github.com/gounix/gosecret"
	"time"
)

const sleepSeconds = 10
var   builderErrors = []string{
	"OK",
	"Environment not set",
	"Registry login failed",
	"Git clone failed",
	"Git checkout failed",
	"Git directory not found",
	"Make failed",
	"Buildah pull failed",
	"Buildah push failed",
}

// createJobSpec returns a job object that can be applied to cluster
// It'll return the yaml example to k8s job object
func createJobSpec(name string, regsync resources.RegsyncT, tag string, srcreg resources.RegistryT, dstreg resources.RegistryT, srcCred gosecret.RegCredT, dstCred gosecret.RegCredT) *batchv1.Job {
	var (
		trueVal           = true
		zeroVal     int32 = 0
		ttl         int32 = 259200 // seconds in 3 days
		env         []corev1.EnvVar
		authEnv     []corev1.EnvVar
	)

	// add current timestamp, as job name should be unique
	name = fmt.Sprintf("%s-%d", name, time.Now().UTC().UnixMilli())

	env = []corev1.EnvVar{
		// info from client-go applyconfigurations/internal/internal.go
		{Name: "SRC_REGISTRY", Value: srcreg.Spec.Host},
		{Name: "SRC_TLS_VERIFY", Value: fmt.Sprintf("%t", srcreg.Spec.TlsVerify)},
		{Name: "SRC_IMAGE", Value: regsync.Spec.Src.Image},
		{Name: "SRC_IMAGE_VERSION", Value: tag},
		{Name: "DST_REGISTRY", Value: dstreg.Spec.Host},
		{Name: "DST_TLS_VERIFY", Value: fmt.Sprintf("%t", dstreg.Spec.TlsVerify)},
		{Name: "DST_IMAGE", Value: regsync.Spec.Target.Image},
		{Name: "SRC_REGISTRY_AUTHENTICATED", Value: fmt.Sprintf("%t", srcreg.Spec.Authenticated)},
		{Name: "DST_REGISTRY_AUTHENTICATED", Value: fmt.Sprintf("%t", dstreg.Spec.Authenticated)},
	}
	if srcreg.Spec.Authenticated == true {
		authEnv = []corev1.EnvVar{
			{Name: "SRC_REGISTRY_USER", Value: srcCred.User},
			{Name: "SRC_REGISTRY_PASSWORD", Value: srcCred.Passwd},
		}
		env = append(env, authEnv...)
	}
	if dstreg.Spec.Authenticated == true {
		authEnv = []corev1.EnvVar{
			{Name: "DST_REGISTRY_USER", Value: dstCred.User},
			{Name: "DST_REGISTRY_PASSWORD", Value: dstCred.Passwd},
		}
		env = append(env, authEnv...)
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           fmt.Sprintf("%s/%s:%s", environ.Env.SyncerRepo, environ.Env.SyncerImage, environ.Env.SyncerTag),
							ImagePullPolicy: "Always",
							Env:             env,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &trueVal,
								ReadOnlyRootFilesystem: &trueVal,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
									"memory": resource.MustParse("256Mi"),
								},
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
									"memory": resource.MustParse("256Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{
									Name:      "run",
									MountPath: "/run",
								},
								corev1.VolumeMount{
									Name:      "vartmp",
									MountPath: "/var/tmp",
								},
								corev1.VolumeMount{
									Name:      "varlibcontainers",
									MountPath: "/var/lib/containers",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "vartmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						corev1.Volume{
							Name: "varlibcontainers",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						corev1.Volume{
							Name: "run",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			BackoffLimit: &zeroVal,
			TTLSecondsAfterFinished : &ttl,
		},
	}
}

func getPodExitCode(clientset *kubernetes.Clientset, jobName string) (int32, error) {
	pods, err := clientset.CoreV1().Pods(environ.Env.SyncerNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		slog.Error("jobs.getPodExitCode clientset.CoreV1", "err", err)
		return 0, err
	}
	for _, pod := range pods.Items {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "Job" && owner.Name == jobName {
				if pod.Status.Message != "" {
					return 0, errors.New(pod.Status.Message)
				}

				ptr := pod.Status.ContainerStatuses[0].State.Terminated
				if ptr != nil {
					exitCode := (*ptr).ExitCode
					slog.Info("jobs.getPodExitCode", "code", exitCode)
					return exitCode, nil
				} else {
					return 0, errors.New("Cannot get exit code of pod")
				}
			}
		}

	}
	slog.Error("jobs.getPodExitCode", "pod", "not found")
	return 0, errors.New("pod not found")
}

func waitForJob(clientset *kubernetes.Clientset, jobName string) error {

	for true {
		job, err := clientset.BatchV1().Jobs(environ.Env.SyncerNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if job.Status.Succeeded > 0 {
			slog.Info("jobs.waitForJob succeeded", "job", jobName)
			return nil // Job ran successfully
		}
		if job.Status.Failed > 0 {
			slog.Error("jobs.waitForJob failed", "job", jobName)
			exitCode, err := getPodExitCode(clientset, jobName)
			if err != nil {
				return err
			}
			return errors.New(builderErrors[exitCode])
		}
		if job.Status.Active == 0 {
			slog.Info("jobs.waitForJob not started", "job", jobName)
		} else {
			slog.Info("jobs.waitForJob still running", "job", jobName)
		}
		time.Sleep(sleepSeconds * time.Second)
	}
	return nil // unreachable code
}

func RunSyncJob(regsync resources.RegsyncT, tag string, srcreg resources.RegistryT, dstreg resources.RegistryT, srcCred gosecret.RegCredT, dstCred gosecret.RegCredT) error {

	// get job spec
	job := createJobSpec("syncer", regsync, tag, srcreg, dstreg, srcCred, dstCred)

	// create a client for default namespace
	jobClient := gok8s.GetClientSet().BatchV1().Jobs(environ.Env.SyncerNamespace)

	// trigger the job
	_, err := jobClient.Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating job: %w", err)
	}

	slog.Info("Job has been created successfully", "name", job.Name)

	return waitForJob(gok8s.GetClientSet(), job.ObjectMeta.Name)
}
