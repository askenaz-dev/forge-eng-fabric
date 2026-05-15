package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/forge-eng-fabric/services/workflow-runtime/internal/runtime"
)

// envBool returns true when the env var is set to a truthy value. The
// active-registry-gateways `gateway.enforced` flag is per-Tenant in the
// product but at the runtime process layer we surface it as a single
// flag — operator deploys a separate replica set per Tenant cohort, or
// flips the flag globally and uses NetworkPolicy + OPA bundles for the
// per-Tenant nuance.
func envBool(name string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}

func main() {
	mcpClient := buildMCPClient()
	a2aClient := buildA2AClient()
	enforced := envBool("GATEWAY_ENFORCED", false)
	registry := runtime.NewActivityRegistryWithOptions(runtime.RegistryOptions{
		MCP:      mcpClient,
		A2A:      a2aClient,
		Enforced: enforced,
	})
	engine := runtime.NewInMemoryEngine(registry, runtime.LogSink{})
	svc := runtime.NewService(engine, nil)

	mux := http.NewServeMux()
	runtime.NewHandler(svc).Mount(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8093"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("workflow-runtime listening on %s gateway.enforced=%v mcp_client=%v a2a_client=%v",
		addr, enforced, mcpClient != nil, a2aClient != nil)
	log.Fatal(server.ListenAndServe())
}

// buildMCPClient constructs the gateway client at process boot. When
// MCP_GATEWAY_URL is unset, the function returns nil — the runtime then
// uses its legacy in-process stub if `gateway.enforced=false`, or refuses
// to dispatch MCP steps if `gateway.enforced=true`.
//
// The real implementation lives in pkg/mcp-shim. To keep the runtime's
// module graph small we resolve the client via a tiny inline HTTP call
// that implements the MCPGatewayClient interface. Production binaries
// can swap this with a direct mcp-shim import — the implementations are
// wire-compatible.
func buildMCPClient() runtime.MCPGatewayClient {
	url := os.Getenv("MCP_GATEWAY_URL")
	if url == "" {
		return nil
	}
	return newInlineHTTPClient(url, "/v1/gw/mcp", os.Getenv("RUNTIME_IDENTITY_TOKEN"))
}

func buildA2AClient() runtime.A2AGatewayClient {
	url := os.Getenv("A2A_GATEWAY_URL")
	if url == "" {
		return nil
	}
	return newInlineHTTPClient(url, "/v1/gw/a2a", os.Getenv("RUNTIME_IDENTITY_TOKEN"))
}
