ARG GOVERSION
ARG REGSYNC_VERSION

FROM docker.io/library/golang:${GOVERSION}-alpine AS build
ARG GOVERSION
ARG REGSYNC_VERSION
WORKDIR /
COPY ./ /

RUN go mod edit -C main -go $GOVERSION
RUN go mod tidy -C main
RUN sed -i s/Development-version/${REGSYNC_VERSION}/ /main/main.go
RUN sed -i s/Golang-version/${GOVERSION}/ /main/main.go

RUN go build -C main -o regsync main.go

FROM docker.io/library/golang:${GOVERSION}-alpine
ARG GOVERSION

COPY --from=build ./main/regsync /regsync

WORKDIR /
CMD [ "/regsync" ]

