# IAM Docker [![Build Status](https://travis-ci.org/swipely/iam-docker.svg?branch=master)](https://travis-ci.org/swipely/iam-docker)

This project allows Docker containers to use different EC2 instance roles from their host.
You can pull release images from [Docker Hub](https://hub.docker.com/r/swipely/iam-docker/).

![Example gif](https://s3.amazonaws.com/swipely-pub/public-images/iam-docker-latest.gif)

## Motivation

When running applications in EC2, [IAM roles](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html) may only be assigned at the instance level.
Assuming that there's only one application running per instance, this works very well.

[Docker](http://github.com/docker/docker) and Amazon's [Elastic Container Sevice (ECS)](https://aws.amazon.com/ecs/) have made it cost effective and convenient to run a container cluster, which could contain any number of applications.
ECS clusters run on plain old EC2 instances, meaning that they can only have one IAM role per instance.
Developers must then choose between running one cluster with a wide set of permissions, or running a different cluster for each permission set.
The former allows you to run multiple applications on a single instance at the cost of security, while the later is optimally secure at the cost of instance count.
Using `iam-docker`, a single server can run multiple applications without paying a security penalty.

Note that `iam-docker` doesn't necessarily need to be used with ECS – or even EC2. Any machine running Docker can run it.

## Usage

Setup an instance IAM role that can perform [`sts:assume-role`](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html) on the roles you'd like to assume.
Also ensure that the assumed roles have a Trust Relationship which allows them to be assumed by the root role.
See this [StackOverflow post](http://stackoverflow.com/a/33850060) for more details.

Start an EC2 instance with that role, then pull and run the image:

```bash
$ docker pull swipely/iam-docker:latest
$ docker run --volume /var/run/docker.sock:/var/run/docker.sock --restart=always --net=host swipely/iam-docker:latest
```

For use outside EC2, set up an IAM user that can assume the appropriate roles, generate API credentials for that user, and pass those credentials to `iam-docker` via the `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables. If your containers require access to other parts of the EC2 metadata API, use `iam-docker -meta-data-api http://<target>` to proxy to the mock metadata service of your choosing.

If you do not want your container to be able to access other AWS metadata endpoints, such as the instance's user data, pass the `--disable-upstream` flag.

Determine the network interface of the Docker network you'd like to proxy (default is `bridge`).
Note that this can be done for an arbitrary number of networks.

```bash
$ export NETWORK="bridge"
$ export PORT="8080"
$ export INTERFACE="$(docker network inspect -f '{{index .Options "com.docker.network.bridge.name"}}' "$NETWORK")"
```

Note: if any of the above commands don't work, please open an issue.
It's likely that Docker updated the output of `docker network inspect`.

Forward requests coming from your Docker network(s) to the running agent:

```bash
$ sudo iptables -t nat \
                -I PREROUTING \
                -p tcp \
                -d 169.254.169.254 \
                --dport 80 \
                -j REDIRECT \
                --to-ports "$PORT" \
                -i "$INTERFACE"
```

When starting containers, set their `com.swipely.iam-docker.iam-profile` (an optionally, `com.swipely.iam-docker.iam-externalid` if
your role requires an external ID) label:

```bash
$ export IMAGE="ubuntu:latest"
$ export PROFILE="arn:aws:iam::1234123412:role/some-role"
$ docker run --label com.swipely.iam-docker.iam-profile="$PROFILE" "$IMAGE"
```

Alternately, set the `IAM_ROLE` (and optionally `IAM_ROLE_EXTERNALID`) environment variable:

```bash
$ export IMAGE="ubuntu:latest"
$ export PROFILE="arn:aws:iam::1234123412:role/some-role"
$ docker run -e IAM_ROLE="$PROFILE" "$IMAGE"
```

## How it works

The application listens to the [Docker events stream](https://docs.docker.com/engine/reference/commandline/events/) for container start events.
When a container is started with a `com.swipely.iam-docker.iam-profile` label, the application assumes that role (if possible).
When the container makes an [EC2 Metadata API](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html), it's forwarded to the application because of the `iptables` rule above.
If the request is for IAM credentials, the application intercepts that and determines which credentials should be passed back to the container.

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
