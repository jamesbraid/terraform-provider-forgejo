package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func personalAccessTokenTestResource(t *testing.T, handler http.HandlerFunc) (*personalAccessTokenResource, tfsdk.State) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := forgejo.NewClient(server.URL, forgejo.SetHTTPClient(server.Client()), forgejo.SetForgejoVersion("15.0.0"))
	require.NoError(t, err)
	r := &personalAccessTokenResource{client: client}
	var schema resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	require.False(t, schema.Diagnostics.HasError(), "%v", schema.Diagnostics)
	return r, tfsdk.State{Schema: schema.Schema}
}

func TestPersonalAccessTokenResourceReadByID(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		tokens string
		absent bool
	}{
		{name: "unchanged", tokens: `[{"id":7,"name":"managed","token_last_eight":"12345678","scopes":["read:user"]}]`},
		{name: "same name replacement", tokens: `[{"id":8,"name":"managed","token_last_eight":"87654321","scopes":["all"]}]`, absent: true},
		{name: "missing token", tokens: `[]`, absent: true},
		{name: "duplicate names", tokens: `[{"id":8,"name":"managed"},{"id":7,"name":"managed","token_last_eight":"12345678","scopes":["read:user"]}]`},
		{name: "case distinct names", tokens: `[{"id":8,"name":"MANAGED"},{"id":7,"name":"managed","token_last_eight":"12345678","scopes":["read:user"]}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, state := personalAccessTokenTestResource(t, func(w http.ResponseWriter, req *http.Request) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "/api/v1/users/test/tokens", req.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				var tokens []forgejo.AccessToken
				require.NoError(t, json.Unmarshal([]byte(test.tokens), &tokens))
				w.Header().Set("X-Total-Count", strconv.Itoa(len(tokens)))
				fmt.Fprint(w, test.tokens)
			})
			data := personalAccessTokenResourceModel{
				User: types.StringValue("test"), ID: types.Int64Value(7),
				Name: types.StringValue("managed"), Token: types.StringValue("creation-time-token"),
				Scopes: types.SetNull(types.StringType),
			}
			require.False(t, state.Set(t.Context(), &data).HasError())
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			require.Equal(t, test.absent, response.State.Raw.IsNull())
			if test.absent {
				return
			}
			var refreshed personalAccessTokenResourceModel
			require.False(t, response.State.Get(t.Context(), &refreshed).HasError())
			require.Equal(t, int64(7), refreshed.ID.ValueInt64())
			require.Equal(t, "managed", refreshed.Name.ValueString())
			require.Equal(t, "creation-time-token", refreshed.Token.ValueString())
			require.Equal(t, "12345678", refreshed.TokenLastEight.ValueString())
			var scopes []string
			require.False(t, refreshed.Scopes.ElementsAs(t.Context(), &scopes, false).HasError())
			require.Equal(t, []string{"read:user"}, scopes)
		})
	}
}

func TestPersonalAccessTokenResourceReadPaginates(t *testing.T) {
	for _, test := range []struct {
		name      string
		pages     []string
		total     string
		status    int
		absent    bool
		fail      bool
		wantPages int
	}{
		{name: "server cap below requested limit", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":7,"name":"managed","scopes":["all"]}]`}, total: "2", wantPages: 2},
		{name: "missing across all pages", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":9,"name":"other"}]`}, total: "2", absent: true, wantPages: 2},
		{name: "empty list with count", total: "0", absent: true, wantPages: 1},
		{name: "missing count", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":7,"name":"managed","scopes":["all"]}]`}, wantPages: 3},
		{name: "malformed count", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":7,"name":"managed","scopes":["all"]}]`}, total: "invalid", wantPages: 3},
		{name: "negative count", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":7,"name":"managed","scopes":["all"]}]`}, total: "-1", wantPages: 3},
		{name: "overflowed count", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":7,"name":"managed","scopes":["all"]}]`}, total: "99999999999999999999999999999", wantPages: 3},
		{name: "count below returned results", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":7,"name":"managed","scopes":["all"]}]`}, total: "0", wantPages: 3},
		{name: "empty final page without count", pages: []string{`[{"id":8,"name":"managed"}]`}, absent: true, wantPages: 2},
		{name: "premature empty page", pages: []string{`[{"id":8,"name":"managed"}]`}, total: "2", fail: true, wantPages: 2},
		{name: "repeated page", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":8,"name":"managed"}]`}, fail: true, wantPages: 2},
		{name: "repeated page with count", pages: []string{`[{"id":8,"name":"managed"}]`, `[{"id":8,"name":"managed"}]`}, total: "2", fail: true, wantPages: 2},
		{name: "later page forbidden", pages: []string{`[{"id":8,"name":"managed"}]`}, total: "2", status: http.StatusForbidden, fail: true, wantPages: 2},
		{name: "later page missing", pages: []string{`[{"id":8,"name":"managed"}]`}, total: "2", status: http.StatusNotFound, fail: true, wantPages: 2},
		{name: "later page fails after match", pages: []string{`[{"id":7,"name":"managed"}]`}, total: "2", status: http.StatusInternalServerError, fail: true, wantPages: 2},
		{name: "later page fails without count", pages: []string{`[{"id":7,"name":"managed"}]`}, status: http.StatusInternalServerError, fail: true, wantPages: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, requests := personalAccessTokenPages(t, test.pages, test.total, test.status)
			r, state := personalAccessTokenTestResource(t, handler)
			data := personalAccessTokenResourceModel{
				User: types.StringValue("test"), ID: types.Int64Value(7),
				Name: types.StringValue("managed"), Token: types.StringValue("creation-time-token"),
				Scopes: types.SetNull(types.StringType),
			}
			require.False(t, state.Set(t.Context(), &data).HasError())
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			var expected []string
			for page := 1; page <= test.wantPages; page++ {
				expected = append(expected, fmt.Sprintf("limit=50&page=%d", page))
			}
			require.Equal(t, expected, *requests)
			require.Equal(t, test.fail, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			if test.fail {
				require.Equal(t, "Unable to list personal access tokens", response.Diagnostics.Errors()[0].Summary())
				require.True(t, state.Raw.Equal(response.State.Raw))
				return
			}
			require.Equal(t, test.absent, response.State.Raw.IsNull())
			if test.absent {
				return
			}
			require.False(t, response.State.Get(t.Context(), &data).HasError())
			require.Equal(t, int64(7), data.ID.ValueInt64())
			require.Equal(t, "creation-time-token", data.Token.ValueString())
		})
	}
}

func personalAccessTokenPages(t *testing.T, pages []string, total string, status int) (http.HandlerFunc, *[]string) {
	t.Helper()
	var requests []string
	return func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, req.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		if total != "" {
			w.Header().Set("X-Total-Count", total)
		}
		page, err := strconv.Atoi(req.URL.Query().Get("page"))
		if err != nil || page < 1 || req.URL.Query().Get("limit") != "50" {
			t.Errorf("unexpected pagination request: %s", req.URL.RawQuery)
			http.Error(w, "unexpected pagination request", http.StatusBadRequest)
			return
		}
		if page == 2 && status != 0 {
			http.Error(w, "cannot list tokens", status)
			return
		}
		if page > len(pages) {
			fmt.Fprint(w, `[]`)
			return
		}
		fmt.Fprint(w, pages[page-1])
	}, &requests
}

func TestPersonalAccessTokenResourceImportPaginates(t *testing.T) {
	for _, test := range []struct {
		name      string
		firstPage string
		status    int
		summary   string
	}{
		{name: "exact name on second page", firstPage: `[{"id":8,"name":"MANAGED"}]`},
		{name: "duplicate across pages", firstPage: `[{"id":8,"name":"managed"}]`, summary: "Ambiguous personal access token name"},
		{name: "later page fails after match", firstPage: `[{"id":8,"name":"managed"}]`, status: http.StatusInternalServerError, summary: "Unable to list personal access tokens"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, requests := personalAccessTokenPages(t, []string{test.firstPage, `[{"id":7,"name":"managed","scopes":["all"]}]`}, "2", test.status)
			r, state := personalAccessTokenTestResource(t, handler)
			response := resource.ImportStateResponse{State: state}
			r.ImportState(t.Context(), resource.ImportStateRequest{ID: "test/managed"}, &response)
			require.Equal(t, []string{"limit=50&page=1", "limit=50&page=2"}, *requests)
			require.Equal(t, test.summary != "", response.Diagnostics.HasError(), "%v", response.Diagnostics)
			if test.summary != "" {
				require.Equal(t, test.summary, response.Diagnostics.Errors()[0].Summary())
				return
			}
			var data personalAccessTokenResourceModel
			require.False(t, response.State.Get(t.Context(), &data).HasError())
			require.Equal(t, int64(7), data.ID.ValueInt64())
			require.True(t, data.Token.IsNull())
		})
	}
}

func TestPersonalAccessTokenResourceReadErrorPreservesState(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			r, state := personalAccessTokenTestResource(t, func(w http.ResponseWriter, req *http.Request) {
				http.Error(w, "cannot list tokens", status)
			})
			data := personalAccessTokenResourceModel{
				User: types.StringValue("test"), ID: types.Int64Value(7),
				Name: types.StringValue("managed"), Token: types.StringValue("creation-time-token"),
				Scopes: types.SetNull(types.StringType),
			}
			require.False(t, state.Set(t.Context(), &data).HasError())
			response := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
			require.True(t, response.Diagnostics.HasError())
			require.True(t, state.Raw.Equal(response.State.Raw))
		})
	}
}

func TestPersonalAccessTokenResourceImport(t *testing.T) {
	for _, test := range []struct {
		name   string
		id     string
		tokens string
		fail   bool
	}{
		{name: "exact name", id: "test/managed", tokens: `[{"id":8,"name":"MANAGED"},{"id":7,"name":"managed","scopes":["all"]}]`},
		{name: "missing", id: "test/managed", tokens: `[]`, fail: true},
		{name: "different case", id: "test/managed", tokens: `[{"id":8,"name":"MANAGED"}]`, fail: true},
		{name: "duplicate names", id: "test/managed", tokens: `[{"id":8,"name":"managed"},{"id":7,"name":"managed"}]`, fail: true},
		{name: "invalid format", id: "test", fail: true},
		{name: "missing user", id: "/managed", fail: true},
		{name: "missing name", id: "test/", fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, state := personalAccessTokenTestResource(t, func(w http.ResponseWriter, req *http.Request) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "/api/v1/users/test/tokens", req.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				var tokens []forgejo.AccessToken
				require.NoError(t, json.Unmarshal([]byte(test.tokens), &tokens))
				w.Header().Set("X-Total-Count", strconv.Itoa(len(tokens)))
				fmt.Fprint(w, test.tokens)
			})
			response := resource.ImportStateResponse{State: state}
			r.ImportState(t.Context(), resource.ImportStateRequest{ID: test.id}, &response)
			require.Equal(t, test.fail, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			if test.fail {
				return
			}
			var data personalAccessTokenResourceModel
			require.False(t, response.State.Get(t.Context(), &data).HasError())
			require.Equal(t, int64(7), data.ID.ValueInt64())
			require.Equal(t, "managed", data.Name.ValueString())
			require.Equal(t, "test", data.User.ValueString())
			require.True(t, data.Token.IsNull())
			read := resource.ReadResponse{State: response.State}
			r.Read(t.Context(), resource.ReadRequest{State: response.State}, &read)
			require.False(t, read.Diagnostics.HasError(), "%v", read.Diagnostics)
			require.False(t, read.State.Get(t.Context(), &data).HasError())
			require.Equal(t, int64(7), data.ID.ValueInt64())
			require.True(t, data.Token.IsNull())
		})
	}
}

func TestPersonalAccessTokenResourceDeleteByID(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests []string
			r, state := personalAccessTokenTestResource(t, func(w http.ResponseWriter, req *http.Request) {
				requests = append(requests, req.Method+" "+req.URL.Path)
				w.WriteHeader(status)
			})
			data := personalAccessTokenResourceModel{
				User: types.StringValue("test"), ID: types.Int64Value(7), Name: types.StringValue("managed"),
				Scopes: types.SetNull(types.StringType),
			}
			require.False(t, state.Set(t.Context(), &data).HasError())
			var response resource.DeleteResponse
			r.Delete(t.Context(), resource.DeleteRequest{State: state}, &response)
			require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			require.Equal(t, []string{"DELETE /api/v1/users/test/tokens/7"}, requests)
		})
	}
}

func TestPersonalAccessTokenResourceCreateAndRefresh(t *testing.T) {
	r, state := personalAccessTokenTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, "/api/v1/users/test/tokens", req.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case http.MethodPost:
			var options forgejo.CreateAccessTokenOption
			require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
			require.Equal(t, "managed", options.Name)
			fmt.Fprint(w, `{"id":7,"name":"managed","sha1":"creation-time-token","token_last_eight":"12345678","scopes":["all"]}`)
		case http.MethodGet:
			w.Header().Set("X-Total-Count", "1")
			fmt.Fprint(w, `[{"id":7,"name":"managed","token_last_eight":"12345678","scopes":["all"]}]`)
		default:
			t.Errorf("unexpected method: %s", req.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	scopes, diags := types.SetValueFrom(t.Context(), types.StringType, []string{"all"})
	require.False(t, diags.HasError())
	data := personalAccessTokenResourceModel{
		User: types.StringValue("test"), Name: types.StringValue("managed"), Scopes: scopes,
	}
	plan := tfsdk.Plan{Schema: state.Schema}
	require.False(t, plan.Set(t.Context(), &data).HasError())
	created := resource.CreateResponse{State: state}
	r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &created)
	require.False(t, created.Diagnostics.HasError(), "%v", created.Diagnostics)
	read := resource.ReadResponse{State: created.State}
	r.Read(t.Context(), resource.ReadRequest{State: created.State}, &read)
	require.False(t, read.Diagnostics.HasError(), "%v", read.Diagnostics)
	require.False(t, read.State.Get(t.Context(), &data).HasError())
	require.Equal(t, int64(7), data.ID.ValueInt64())
	require.Equal(t, "creation-time-token", data.Token.ValueString())
}
