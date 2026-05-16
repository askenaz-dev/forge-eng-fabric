module github.com/forge-eng-fabric/services/trigger-router

go 1.22

require (
	github.com/forge-eng-fabric/pkg/workflow v0.0.0
	github.com/google/uuid v1.6.0
	github.com/robfig/cron/v3 v3.0.1
)

replace github.com/forge-eng-fabric/pkg/workflow => ../../pkg/workflow
