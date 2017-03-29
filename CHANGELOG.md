# CHANGELOG
## v1.4.0

* Support IAM roles that have [ExternalIds](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user_externalid.html) (@iderdik)

## v1.1.0

* Support IAM profiles specified via environment variables (@willglynn)

## v1.0.0

* First versioned release of iam-docker
* Listens to events from the Docker daemon to add containers in real time
* Keeps credentials fresh within a 30 minute window
* Supports proxying the metadata API for containers running in any network
