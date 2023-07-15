FROM golang:1.20-alpine as builder

RUN apk add --no-cache git gcc musl-dev libseccomp-dev
ENV GO111MODULE=on CGO_ENABLED=1
WORKDIR /work
ADD wl-file-activity-tracer.go go.mod go.sum /work/
RUN go build -o /work/wlftracer wl-file-activity-tracer.go

# Path: Containerfile
FROM alpine
RUN apk add --no-cache libseccomp
COPY --from=builder /work/wlftracer /usr/bin/wlftracer
ENTRYPOINT ["/usr/bin/wlftracer"]
