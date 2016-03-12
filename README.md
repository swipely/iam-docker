# IAM Docker

This project allows Docker containers to use different EC2 instance roles.
It's still pre-release, so expect the interface to change.

## Usage

* Build a release Docker image using `make docker`
* Load that image on a host which can assume roles: `docker save iam-docker:latest | ssh $user@$host 'docker load'`
* Run the Docker image: `docker run --volume /var/lib/docker:/var/lib/docker --port 8080:8080 iam-docker:latest`
* Determine the gateway for the Docker network you'd like to proxy (default is bridge): `docker network inspect $network | grep Gateway | cut -d '"' -f 4`
* Forward requests from the Docker bridge going to the metadata api to the proxy: `iptables -t nat -I PREROUTING -p tcp -d 169.254.169.254 --dport 80 -j DNAT --to-destination $gateway:8080 -i $network`
* When starting containers, set an `IAM_ROLE` environment variable: `docker run -it -e IAM_PROFILE=arn:aws:iam::1234123412:role/some-role $image`

## How it works

The application listens to the [Docker events stream](https://docs.docker.com/engine/reference/commandline/events/) for container start events.
When a container is started with an `IAM_PROFILE` environment variable, the application assumes that role (if possible).
When the container makes an [EC2 Metadata API](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) EC2 metadata API request, it's forwarded to the application because of the `iptables` rule above.
If the request is for IAM credentials, the application intercepts that and determines which credentials should be passed back to the container.
Otherwise, it acts as a reverse proxy to the real metadata API.

All credentials are kept fresh, so there should be little latency when making API requests.

## Development

All development commands can be found in the `Makefile`.
Commonly used commands:

* `make test` - run the application tests
* `make docker` - build a release Docker image
* `make test-in-docker` - run the tests in Docker

All source files are organized into differente modules in the `src/` directory.
