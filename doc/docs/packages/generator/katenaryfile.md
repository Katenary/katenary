<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# katenaryfile

```go
import "katenary/generator/katenaryfile"
```

Package katenaryfile is a package for reading and writing katenary files.

A katenary file, named "katenary.yml" or "katenary.yaml", is a file where you can define the configuration of the conversion avoiding the use of labels in the compose file.

Formely, the file describe the same structure as in labels, and so that can be validated and completed by LSP. It also ease the use of katenary.

## func [GenerateSchema](<https://github.com/katenary/katenary/blob/develop/generator/katenaryfile/main.go#L137>)

```go
func GenerateSchema() string
```

GenerateSchema generates the schema for the katenary.yaml file.

<a name="OverrideWithConfig"></a>
## func [OverrideWithConfig](<https://github.com/katenary/katenary/blob/develop/generator/katenaryfile/main.go#L49>)

```go
func OverrideWithConfig(project *types.Project)
```

OverrideWithConfig overrides the project with the katenary.yaml file. It will set the labels of the services with the values from the katenary.yaml file. It work in memory, so it will not modify the original project.

<a name="Service"></a>
## type [Service](<https://github.com/katenary/katenary/blob/develop/generator/katenaryfile/main.go#L27-L44>)

Service is a struct that contains the service configuration for katenary

```go
type Service struct {
    MainApp         *bool                          `json:"main-app,omitempty" jsonschema:"title=Is this service the main application"`
    Values          []StringOrMap                  `json:"values,omitempty" jsonschema:"description=Environment variables to be set in values.yaml with or without a description"`
    Secrets         *labelstructs.Secrets          `json:"secrets,omitempty" jsonschema:"title=Secrets,description=Environment variables to be set as secrets"`
    Ports           *labelstructs.Ports            `json:"ports,omitempty" jsonschema:"title=Ports,description=Ports to be exposed in services"`
    Ingress         *labelstructs.Ingress          `json:"ingress,omitempty" jsonschema:"title=Ingress,description=Ingress configuration"`
    HealthCheck     *labelstructs.HealthCheck      `json:"health-check,omitempty" jsonschema:"title=Health Check,description=Health check configuration that respects the kubernetes api"`
    SamePod         *string                        `json:"same-pod,omitempty" jsonschema:"title=Same Pod,description=Service that should be in the same pod"`
    Description     *string                        `json:"description,omitempty" jsonschema:"title=Description,description=Description of the service that will be injected in the values.yaml file"`
    Ignore          *bool                          `json:"ignore,omitempty" jsonschema:"title=Ignore,description=Ignore the service in the conversion"`
    Dependencies    []labelstructs.Dependency      `json:"dependencies,omitempty" jsonschema:"title=Dependencies,description=Services that should be injected in the Chart.yaml file"`
    ConfigMapFile   *labelstructs.ConfigMapFile    `json:"configmap-files,omitempty" jsonschema:"title=ConfigMap Files,description=Files that should be injected as ConfigMap"`
    MapEnv          *labelstructs.MapEnv           `json:"map-env,omitempty" jsonschema:"title=Map Env,description=Map environment variables to another value"`
    CronJob         *labelstructs.CronJob          `json:"cron-job,omitempty" jsonschema:"title=Cron Job,description=Cron Job configuration"`
    EnvFrom         *labelstructs.EnvFrom          `json:"env-from,omitempty" jsonschema:"title=Env From,description=Inject environment variables from another service"`
    ExchangeVolumes []*labelstructs.ExchangeVolume `json:"exchange-volumes,omitempty" jsonschema:"title=Exchange Volumes,description=Exchange volumes between services"`
    ValuesFrom      *labelstructs.ValueFrom        `json:"values-from,omitempty" jsonschema:"title=Values From,description=Inject values from another service (secret or configmap environment variables)"`
}
```

<a name="StringOrMap"></a>
## type [StringOrMap](<https://github.com/katenary/katenary/blob/develop/generator/katenaryfile/main.go#L24>)

StringOrMap is a struct that can be either a string or a map of strings. It's a helper struct to unmarshal the katenary.yaml file and produce the schema

```go
type StringOrMap any
```

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
