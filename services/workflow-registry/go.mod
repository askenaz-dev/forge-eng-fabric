module github.com/forge-eng-fabric/services/workflow-registry

go 1.22

require (
	github.com/forge-eng-fabric/pkg/workflow v0.0.0
	github.com/google/uuid v1.6.0
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

replace github.com/forge-eng-fabric/pkg/workflow => ../../pkg/workflow
