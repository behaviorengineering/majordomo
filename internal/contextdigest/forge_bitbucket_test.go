package contextdigest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListBitbucketComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/1.0/projects/PROJ/repos/repo/pull-requests/7/activities" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{
				{
					"action": "COMMENTED",
					"createdDate": 123,
					"user": map[string]string{"name": "alice"},
					"comment": map[string]string{"text": "@majordomo done"},
				},
				{"action": "REVIEWED"},
			},
			"isLastPage": true,
		})
	}))
	defer srv.Close()

	f := &Forge{
		SCM: "bitbucket", Owner: "PROJ", Name: "repo", Token: "tok", BaseURL: srv.URL,
		Client: srv.Client(),
	}
	comments, err := f.ListPRComments("7")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].Body != "@majordomo done" {
		t.Fatalf("comments=%+v", comments)
	}
}
