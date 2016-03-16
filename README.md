# IAM Docker [![Build Status](https://travis-ci.org/swipely/iam-docker.svg?branch=master)](https://travis-ci.org/swipely/iam-docker)

This project allows Docker containers to use different EC2 instance roles.
You can pull release images from [Docker Hub](https://hub.docker.com/r/swipely/iam-docker/).

[![Example gif](https://s3.amazonaws.com/swipely-pub/public-images/iam-docker.gif)]

## Motivation

When running applications in EC2, [IAM roles](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html) may only be assigned at the instance level.
Assuming that there's only one application running per instance, this works very well.

[Docker](http://github.com/docker/docker) and Amazon's [Elastic Container Sevice (ECS)](https://aws.amazon.com/ecs/) have made it cost effective and convenient to run a container cluster, which could container any number of applications.
ECS clusters run on plain old EC2 instances, meaning that they can only have one IAM role per instance.
Developers must then choose between running one cluster with a wide set of permissions, or running a different cluster for each permission set.
These both have their disadvantages; the former is less secure than normal EC2 instances, while the latter doesn't take full advantage of containerized applications.
Using `iam-docker`, containers can be assigned IAM Roles, allowing one cluster to run an arbitrary number of applications without sacrificing the security of vanilla EC2.

Note that `iam-docker` doesn't necessarily need to be used with ECS -- any EC2 instance running Docker can run use it.

## Usage

Setup an root IAM role that can perform [`sts:assume-role`](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html) on the roles you'd like to assume.
Also ensure that the assumed roles have a Trust Relationship which allows them to be assumed by the root role.
See this [StackOverflow post](http://stackoverflow.com/a/33850060) for more details.

Start an EC2 instance with that role, then pull and run the image:

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
