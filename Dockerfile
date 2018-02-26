FROM golang:1-alpine AS builder

RUN apk add --no-cache curl

# Make a /target directory which we'll copy into the target image as a single layer, and
# populate it with some SSL roots
RUN mkdir -p /target/etc/ssl/certs && \
	curl -s -o /target/etc/ssl/certs/ca-certificates.crt https://curl.haxx.se/ca/cacert.pem

# Copy in the app
ENV CGO_ENABLED=0
WORKDIR /go/src/github.com/swipely/iam-docker/
ADD . .

# Run tests
RUN go test -v ./...

# Build the app via `go install`, copy the binary to /target/iam-docker, and copy the license file
RUN go install ./... && \
	cp /go/bin/src /target/iam-docker && \
	cp LICENSE /target/

# Build the final image
FROM scratch
MAINTAINER Tom Hulihan (hulihan.tom159@gmail.com)
COPY --from=builder /target /
ENTRYPOINT ["/iam-docker"]
