module github.com/forge-eng-fabric/services/app-onboarding

go 1.22

require (
	github.com/forge-eng-fabric/services/scaffolder v0.0.0
	github.com/google/uuid v1.6.0
)

replace github.com/forge-eng-fabric/services/scaffolder => ../scaffolder

require gopkg.in/yaml.v3 v3.0.1 // indirect
