# IAM Docker [![Build Status](https://travis-ci.org/swipely/iam-docker.svg?branch=master)](https://travis-ci.org/swipely/iam-docker)

This project allows Docker containers to use different EC2 instance roles.
You can pull release images from [Docker Hub](https://hub.docker.com/r/swipely/iam-docker/).

## Usage

Pull and run the image:

```bash
$ docker pull swipely/iam-docker:latest
$ docker run --volume /var/run/docker.sock:/var/run/docker.sock --restart=always swipely/iam-docker:latest
```

Determine the gateway IP and network interface of the Docker network you'd like to proxy (default is `bridge`).
Note that this can be done for an arbitrary number of networks.

```bash
$ export NETWORK="bridge"
$ export GATEWAY="$(docker network inspect "$NETWORK" | grep Gateway | cut -d '"' -f 4)"
$ export INTERFACE="br-$(docker network inspect "$NETWORK" | grep Id | cut -d '"' -f 4 | head -c 12)"
```

Forward requests coming from your Docker network(s) to the running agent:

```bash
$ iptables -t nat -I PREROUTING -p tcp -d 169.254.169.254 --dport 80 -j DNAT --to-destination "$GATEWAY":8080 -i "$INTERFACE"
```

When starting containers, set their `IAM_PROFILE` environment variable:

```bash
$ export IMAGE="ubuntu:latest"
$ export PROFILE="arn:aws:iam::1234123412:role/some-role"
$ docker run -e IAM_PROFILE="$PROFILE" "$IMAGE"
```

## How it works

The application listens to the [Docker events stream](https://docs.docker.com/engine/reference/commandline/events/) for container start events.
When a container is started with an `IAM_PROFILE` environment variable, the application assumes that role (if possible).
When the container makes an [EC2 Metadata API](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) EC2 metadata API request, it's forwarded to the application because of the `iptables` rule above.
If the request is for IAM credentials, the application intercepts that and determines which credentials should be passed back to the container.
Otherwise, it acts as a reverse proxy to the real metadata API.

All credentials are kept fresh, so there should be minimal latency when making API requests.

## Development

To build and test, you need to install [Go 1.6](https://golang.org/doc/go1.6) and [`godep`](https://github.com/tools/godep): `go get -u github.com/tools/godep`.

All development commands can be found in the `Makefile`.
Commonly used commands:

* `make get-deps` - install the system dependencies
* `make test` - run the application tests
* `make docker` - build a release Docker image
* `make test-in-docker` - run the tests in Docker

All source code is in the `src/` directory.
