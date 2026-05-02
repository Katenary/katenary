# Basic Usage

Basically, you can use `katenary` to transpose a docker-compose file (or any compose file compatible with
`podman-compose` and `docker-compose`) to a configurable Helm Chart. This resulting helm chart can be installed with
`helm` command to your Kubernetes cluster.

For very basic compose files, without any specific configuration, Katenary will create a working helm chart using the
simple command line:

```bash
katenary convert
```

This will create a `chart` directory with the helm chart inside.

But, in general, you will need to add a few configurations to help Katenary to transpose the compose file to a working
helm chart.

There are two ways to configure Katenary:

- Using the compose files, adding labels to the services
- Using a specific file named `katenary.yaml`

The Katenary file `katenary.yaml` has benefits over the labels in the compose file:

- you can validate the configuration with a schema, and use completion in your editor
- you separate the configuration and leave the compose file "intact"
- the syntax is a bit simpler, instead of using `katenary.v3/xxx: |-` you can use `xxx: ...`

But: **this implies that you have to maintain two files if the compose file changes.**

For example. With "labels", you should do:

```yaml
# in compose file
services:
  webapp:
    image: php:7-apache
    ports:
      - 8080:80
    environment:
      DB_HOST: database
    labels:
    katenary.v3/ingress: |-
      hostname: myapp.example.com
      port: 8080
    katenary.v3/map-env: |-
      DB_HOST: "{{ .Release.Name }}-database"
```

Using a Katenary file, you can do:

```yaml
# in compose file, no need to add labels
services:
  webapp:
    image: php:7-apache
    ports:
      - 8080:80
    environment:
      DB_HOST: database

# in katenary.yaml
webapp:
  ingress:
    hostname: myapp.example.com
    port: 8080

  map-env:
    DB_HOST: "{{ .Release.Name }}-database"
```

!!! Warning "YAML in multiline label"

    Compose only accept text label. So, to put a complete YAML content in the target label,
    you need to use a pipe char (`|` or `|-`) and to **indent** your content.

    For example :

    ```yaml
      labels:
        # your labels
        foo: bar
        # katenary labels with multiline
        katenary.v3/ingress: |-
          hostname: my.website.tld
          port: 80
        katenary.v3/ports: |-
          - 1234
    ```

Katenary transforms compose services this way:

- Takes the service and create a "Deployment" file
- if a port is declared, Katenary creates a service (`ClusterIP`)
- if a port is exposed, Katenary creates a service (`NodePort`)
- environment variables will be stored inside a `ConfigMap`
- image, tags, and ingresses configuration are also stored in `values.yaml` file
- if named volumes are declared, Katenary create `PersistentVolumeClaims` - not enabled in values file
- `depends_on` uses Kubernetes API by default to check if the service endpoint is ready. No port required.
- If you need to create a Kubernetes Service for external access, add the `katenary.v3/ports` label.
  Use label `katenary.v3/depends-on: legacy` to use the old netcat method (requires port).

For any other specific configuration, like binding local files as `ConfigMap`, bind variables, add values with
documentation, etc. You'll need to use labels.

Katenary can also configure containers grouping in pods, declare dependencies, ignore some services, force variables as
secrets, mount files as `configMap`, and many others things. To adapt the helm chart generation, you will need to use
some specific labels.

For more complete label usage, see [the labels page](labels.md).

!!! Info "Overriding file"

    It could be sometimes more convinient to separate the
    configuration related to Katenary inside a secondary file.

    Instead of adding labels inside the `compose.yaml` file,
    you can create a file named `compose.katenary.yaml` and
    declare your labels inside. Katenary will detect it by
    default.

    **No need to precise the file in the command line.**

## Make conversion

After having installed `katenary`, the standard usage is to call:

    katenary convert

It will search standard compose files in the current directory and try to create a helm chart in "chart" directory.

!!! Info

    Katenary uses the compose-go library which respects the Docker and Docker-Compose specification. Keep in mind that
    it will find files exactly the same way as `docker-compose` and `podman-compose` do it.

Of course, you can provide others files than the default with (cumulative) `-c` options:

    katenary convert -c file1.yaml -c file2.yaml

## Some common labels to use

Katenary proposes a lot of labels to configure the helm chart generation, but some are very important.

!!! Info

    For more complete label usage, see [the labels page](labels.md).

### Work with Depends On?

Katenary creates `initContainer` to wait for dependent services to be ready. By default, it uses the Kubernetes API
to check if the service endpoint has ready addresses - no port required.

```yaml
version: "3"

services:
  webapp:
    image: php:8-apache
    depends_on:
      - database

  database:
    image: mariadb
    environment:
      MYSQL_ROOT_PASSWORD: foobar
```

If you need the old netcat-based method (requires port), add the `katenary.v3/depends-on: legacy` label to the dependent service:

```yaml
version: "3"

services:
  webapp:
    image: php:8-apache
    depends_on:
      - database
    labels:
      katenary.v3/depends-on: legacy

  database:
    image: mariadb
    environment:
      MYSQL_ROOT_PASSWORD: foobar
    ports:
      - 3306:3306
```

If you want to create a Kubernetes Service for external access, add the `katenary.v3/ports` label to the service:

```yaml
version: "3"

services:
  webapp:
    image: php:8-apache
    depends_on:
      - database

  database:
    image: mariadb
    environment:
      MYSQL_ROOT_PASSWORD: foobar
    labels:
      katenary.v3/ports:
        - 3306
```

### Declare ingresses

It's very common to have an Ingress resource on web application to deploy on Kubernetes. It allows exposing the
service to the outside of the cluster (you need to install an ingress controller).

Katenary can create this resource for you. You just need to declare the hostname and the port to bind.

```yaml
services:
  webapp:
    image: ...
    ports: 8080:5050
    labels:
      katenary.v3/ingress: |-
        # the target port is 5050 wich is the "service" port
        port: 5050
        hostname: myapp.example.com
```

Note that the port to bind is the one used by the container, not the used locally. This is because Katenary create a
service to bind the container itself.

### Map environment to helm values

A lot of framework needs to receive service host or IP in an environment variable to configure the connection. For
example, to connect a PHP application to a database.

With a compose file, there is no problem as Docker/Podman allows resolving the name by container name:

```yaml
services:
  webapp:
    image: php:7-apache
    environment:
      DB_HOST: database

  database:
    image: mariadb
```

Katenary prefixes the services with `{{ .Release.Name }}` (to make it possible to install the application several times
in a namespace), so you need to "remap" the environment variable to the right one.

```yaml
services:
  webapp:
    image: php:7-apache
    environment:
      DB_HOST: database
    labels:
      katenary.v3/mapenv: |-
        DB_HOST: "{{ .Release.Name }}-database"

  database:
    image: mariadb
```

This label can be used to map others environment for any others reason. E.g. to change an informational environment
variable.

```yaml
services:
  webapp:
    #...
    environment:
      RUNNING: docker
    labels:
      katenary.v3/mapenv: |-
        RUNNING: kubernetes
```

In the above example, `RUNNING` will be set to `kubernetes` when you'll deploy the application with helm, and it's
`docker` for "Podman" and "Docker" executions.
