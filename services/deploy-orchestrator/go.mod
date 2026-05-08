module github.com/forge-eng-fabric/services/deploy-orchestrator

go 1.22

require (
	github.com/forge-eng-fabric/pkg/cosign v0.0.0
	github.com/forge-eng-fabric/pkg/deployers v0.0.0
	github.com/forge-eng-fabric/pkg/runtimes v0.0.0
	github.com/google/uuid v1.6.0
)

replace (
	github.com/forge-eng-fabric/pkg/cosign => ../../pkg/cosign
	github.com/forge-eng-fabric/pkg/deployers => ../../pkg/deployers
	github.com/forge-eng-fabric/pkg/runtimes => ../../pkg/runtimes
)
