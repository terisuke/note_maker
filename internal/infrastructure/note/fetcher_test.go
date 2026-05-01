package note

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestFetchArticlePrefersPublicPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/n/n123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<html><head><meta property="og:title" content="記事タイトル"></head><body><article><p>本文です。</p></article></body></html>`))
	}))
	defer server.Close()

	fetcher := NewFetcherWithClient(mappedClient(server.URL))
	article, err := fetcher.FetchArticle(context.Background(), "https://note.com/user/n/n123")
	if err != nil {
		t.Fatalf("fetch article: %v", err)
	}
	if article.Title != "記事タイトル" {
		t.Fatalf("unexpected title: %q", article.Title)
	}
	if article.Content != "本文です。" {
		t.Fatalf("unexpected content: %q", article.Content)
	}
}

func TestFetchArticleFallsBackToCompatibilityAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/n/n123":
			_, _ = w.Write([]byte(`<html><body><main></main></body></html>`))
		case "/api/v3/notes/n123":
			_, _ = w.Write([]byte(`{"data":{"name":"API Title","body":"<p>API body</p>"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	fetcher := NewFetcherWithClient(mappedClient(server.URL))
	article, err := fetcher.FetchArticle(context.Background(), "https://note.com/user/n/n123")
	if err != nil {
		t.Fatalf("fetch article: %v", err)
	}
	if article.Title != "API Title" || article.Content != "API body" {
		t.Fatalf("unexpected article: %#v", article)
	}
}

func TestFetchUserLatestArticlesUsesRSS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/writer/rss":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel><item><title>One</title><link>https://note.com/writer/n/n1</link><description>desc</description></item></channel></rss>`))
		case "/writer/n/n1":
			_, _ = w.Write([]byte(`<html><body><article><p>RSS article body</p></article></body></html>`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	fetcher := NewFetcherWithClient(mappedClient(server.URL))
	articles, err := fetcher.FetchUserLatestArticles(context.Background(), "writer", 3)
	if err != nil {
		t.Fatalf("fetch user articles: %v", err)
	}
	if len(articles) != 1 || articles[0].Content != "RSS article body" {
		t.Fatalf("unexpected articles: %#v", articles)
	}
}

func mappedClient(target string) *http.Client {
	targetURL, _ := url.Parse(target)
	return &http.Client{Transport: rewriteTransport{target: targetURL, next: http.DefaultTransport}}
}

type rewriteTransport struct {
	target *url.URL
	next   http.RoundTripper
}

func (t rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = t.target.Host
	if strings.HasPrefix(clone.URL.Path, "//") {
		clone.URL.Path = strings.TrimPrefix(clone.URL.Path, "/")
	}
	return t.next.RoundTrip(clone)
}
