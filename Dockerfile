FROM golang:1.6
MAINTAINER Tom Hulihan (hulihan.tom159@gmail.com)
RUN go get github.com/tools/godep
ADD . /go/src/github.com/swipely/iam-docker/
WORKDIR /go/src/github.com/swipely/iam-docker/
RUN godep restore ./src/...
RUN make exe
EXPOSE 8080
CMD /go/src/github.com/swipely/iam-docker/dist/iam-docker
