IMAGE=regsync
IMAGE_VERSION=1.0.0
GOVERSION=1.26.4

.PHONY: dockerhub 

dockerhub:
	buildah build --build-arg GOVERSION=${GOVERSION} --build-arg REGSYNC_VERSION=${IMAGE_VERSION} -t ${IMAGE}:${IMAGE_VERSION} .
	buildah push ${IMAGE}:${IMAGE_VERSION} gounix/${IMAGE}:${IMAGE_VERSION}

