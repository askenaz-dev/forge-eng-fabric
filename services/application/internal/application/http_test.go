package application

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mountTest(t *testing.T) (*httptest.Server, *Service) {
	t.Helper()
	svc, _, _, _ := testSetup(t)
	mux := http.NewServeMux()
	NewHandler(svc).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, svc
}

func TestHTTP_CreateAndGet(t *testing.T) {
	srv, _ := mountTest(t)
	body := bytes.NewBufferString(`{"slug":"hr-portal","name":"HR","owners":["alice"]}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/workspaces/ws-1/apps", body)
	req.Header.Set("X-Forge-Principal", "alice")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created App
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp2, err := http.Get(srv.URL + "/v1/apps/" + created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestHTTP_SlugConflictReturns409(t *testing.T) {
	srv, _ := mountTest(t)
	post := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/workspaces/ws-1/apps",
			bytes.NewBufferString(`{"slug":"hr-portal","name":"HR","owners":["alice"]}`))
		req.Header.Set("X-Forge-Principal", "alice")
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		return resp
	}
	first := post()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first: %d", first.StatusCode)
	}
	first.Body.Close()
	second := post()
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", second.StatusCode)
	}
}

func TestHTTP_ArchiveActionEndpoint(t *testing.T) {
	srv, _ := mountTest(t)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/workspaces/ws-1/apps",
		bytes.NewBufferString(`{"slug":"hr-portal","name":"HR","owners":["alice"]}`))
	req.Header.Set("X-Forge-Principal", "alice")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var created App
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	archReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/apps/"+created.ID+":archive", nil)
	archReq.Header.Set("X-Forge-Principal", "alice")
	archResp, _ := http.DefaultClient.Do(archReq)
	defer archResp.Body.Close()
	if archResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on archive, got %d", archResp.StatusCode)
	}
}
