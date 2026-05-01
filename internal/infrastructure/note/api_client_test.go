package note

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIClientFetchUserArticlesUsesNoteAPIDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/creators/writer/contents":
			_, _ = w.Write([]byte(`{"data":{"contents":[{"key":"n1","name":"One","noteUrl":"https://note.com/writer/n/n1","status":"published"}]}}`))
		case "/api/v3/notes/n1":
			_, _ = w.Write([]byte(`{"data":{"name":"One","noteUrl":"https://note.com/writer/n/n1","body":"<p>最初の段落。</p><p>次の段落。</p>"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewAPIClient(mappedClient(server.URL))
	articles, err := client.FetchUserArticles(context.Background(), "writer", 3)
	if err != nil {
		t.Fatalf("fetch user articles: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("unexpected articles: %#v", articles)
	}
	if articles[0].Content != "最初の段落。\n\n次の段落。" {
		t.Fatalf("paragraphs were not preserved: %q", articles[0].Content)
	}
}
