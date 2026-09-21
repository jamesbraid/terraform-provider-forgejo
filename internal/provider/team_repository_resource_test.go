package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestTeamRepositoryAPILifecycle(t *testing.T) {
	t.Parallel()

	var mutex sync.Mutex
	assigned := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()

		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/repositories/100":
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprint(writer, `{"id":100,"name":"tofu","owner":{"login":"infra"}}`)
		case request.Method == http.MethodPut && request.URL.Path == "/api/v1/teams/12/repos/infra/tofu":
			assigned = true
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPut && request.URL.Path == "/api/v1/teams/14/repos/infra/tofu":
			http.Error(writer, "grant failed", http.StatusInternalServerError)
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/teams/12/repos":
			writer.Header().Set("Content-Type", "application/json")
			if request.URL.Query().Get("page") == "1" {
				writer.Header().Set("Link", "<"+server.URL+"/api/v1/teams/12/repos?page=2&limit=50>; rel=\"next\"")
				fmt.Fprint(writer, `[{"id":99,"name":"deploy","owner":{"login":"infra"}}]`)
			} else if assigned {
				fmt.Fprint(writer, `[{"id":100,"name":"tofu","owner":{"login":"infra"}}]`)
			} else {
				fmt.Fprint(writer, `[]`)
			}
		case request.Method == http.MethodDelete && request.URL.Path == "/api/v1/teams/12/repos/infra/tofu":
			assigned = false
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/teams/13/repos":
			http.Error(writer, "team not found", http.StatusNotFound)
		default:
			http.Error(writer, request.Method+" "+request.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := forgejo.NewClient(server.URL, forgejo.SetHTTPClient(server.Client()), forgejo.SetForgejoVersion("15.0.0"))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if diagnostics := assignTeamRepository(ctx, client, 12, 100); diagnostics.HasError() {
		t.Fatalf("assign diagnostics: %v", diagnostics)
	}
	present, diagnostics := teamHasRepository(ctx, client, 12, 100)
	if diagnostics.HasError() || !present {
		t.Fatalf("read after assign: present=%v diagnostics=%v", present, diagnostics)
	}
	if diagnostics := unassignTeamRepository(client, 12, 100); diagnostics.HasError() {
		t.Fatalf("unassign diagnostics: %v", diagnostics)
	}
	present, diagnostics = teamHasRepository(ctx, client, 12, 100)
	if diagnostics.HasError() || present {
		t.Fatalf("read after unassign: present=%v diagnostics=%v", present, diagnostics)
	}
	present, diagnostics = teamHasRepository(ctx, client, 13, 100)
	if diagnostics.HasError() || present {
		t.Fatalf("read missing team: present=%v diagnostics=%v", present, diagnostics)
	}
	diagnostics = assignTeamRepository(ctx, client, 14, 100)
	if !diagnostics.HasError() {
		t.Fatal("assign API error returned no diagnostics")
	}
	errors := diagnostics.Errors()
	if len(errors) != 1 || errors[0].Summary() != "Unable to add repository to team" {
		t.Fatalf("unexpected assign API diagnostics: %v", diagnostics)
	}
}

func TestUnassignTeamRepositoryForgetsMissingRepository(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/v1/repositories/101" {
			http.Error(writer, request.Method+" "+request.URL.Path, http.StatusInternalServerError)
			return
		}
		http.Error(writer, "repository not found", http.StatusNotFound)
	}))
	defer server.Close()

	client, err := forgejo.NewClient(server.URL, forgejo.SetHTTPClient(server.Client()), forgejo.SetForgejoVersion("15.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := unassignTeamRepository(client, 12, 101); diagnostics.HasError() {
		t.Fatalf("unassign after repository deletion: %v", diagnostics)
	}
}

func TestParseTeamRepositoryImportID(t *testing.T) {
	t.Parallel()

	teamID, repositoryID, err := parseTeamRepositoryImportID("12/100")
	if err != nil || teamID != 12 || repositoryID != 100 {
		t.Fatalf("valid import ID: team=%d repository=%d error=%v", teamID, repositoryID, err)
	}
	for _, id := range []string{"", "12", "12/100/1", "zero/100", "0/100", "12/-1"} {
		if _, _, err := parseTeamRepositoryImportID(id); err == nil {
			t.Errorf("malformed import ID %q returned no error", id)
		}
	}
}
