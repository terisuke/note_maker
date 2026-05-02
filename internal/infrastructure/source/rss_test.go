package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

func TestRSSFetcherParsesRSS2ContentEncoded(t *testing.T) {
	feed := `<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <item>
      <title>最新記事</title>
      <link>https://example.com/post-1</link>
      <guid>post-1</guid>
      <pubDate>Sat, 02 May 2026 12:00:00 +0900</pubDate>
      <content:encoded><![CDATA[<p>本文です。</p><p>続きです。</p>]]></content:encoded>
    </item>
  </channel>
</rss>`
	snapshots, err := parseFeed(strings.NewReader(feed), sourcedomain.Ref{Kind: sourcedomain.KindRSS, URL: "https://example.com/feed.xml"}, 5, testFetchedAt)
	if err != nil {
		t.Fatalf("parse feed: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("len = %d", len(snapshots))
	}
	if snapshots[0].Title != "最新記事" || snapshots[0].Content != "本文です。\n\n続きです。" {
		t.Fatalf("unexpected snapshot: %#v", snapshots[0])
	}
	if snapshots[0].PublishedAt.IsZero() {
		t.Fatalf("published time was not parsed")
	}
}

func TestRSSFetcherParsesAtom(t *testing.T) {
	feed := `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>tag:example.com,2026:1</id>
    <title>Atom記事</title>
    <updated>2026-05-02T12:00:00+09:00</updated>
    <link rel="alternate" href="https://example.com/atom-1" />
    <content type="html"><![CDATA[<p>Atom本文</p>]]></content>
  </entry>
</feed>`
	snapshots, err := parseFeed(strings.NewReader(feed), sourcedomain.Ref{Kind: sourcedomain.KindRSS, URL: "https://example.com/atom.xml"}, 5, testFetchedAt)
	if err != nil {
		t.Fatalf("parse atom: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].URL != "https://example.com/atom-1" || snapshots[0].Content != "Atom本文" {
		t.Fatalf("unexpected snapshots: %#v", snapshots)
	}
}

func TestAuthorStyleFetcherRoutesExplicitSources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zenn-user/feed":
			if r.URL.Query().Get("all") != "1" {
				t.Fatalf("zenn feed should request all=1, got %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel><item><title>Zenn</title><link>https://zenn.dev/zenn-user/articles/a</link><description><![CDATA[<p>Zenn本文</p>]]></description></item></channel></rss>`))
		case "/api/v2/users/qiita-user/items":
			_, _ = w.Write([]byte(`[{"id":"abc","url":"https://qiita.com/qiita-user/items/abc","title":"Qiita","body":"# Qiita本文","created_at":"2026-05-02T00:00:00+09:00","updated_at":"2026-05-02T00:00:00+09:00"}]`))
		case "/feed.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel><item><title>RSS</title><link>https://example.com/rss</link><description><![CDATA[<p>RSS本文</p>]]></description></item></channel></rss>`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	fetcher := NewAuthorStyleFetcherWithClient(mappedClient(server.URL))
	cases := map[string]string{
		"zenn:zenn-user":         "Zenn本文",
		"qiita:qiita-user":       "# Qiita本文",
		"rss:https://x/feed.xml": "RSS本文",
	}
	for selector, wantContent := range cases {
		articles, err := fetcher.FetchUserLatestArticles(context.Background(), selector, 3)
		if err != nil {
			t.Fatalf("%s: fetch latest: %v", selector, err)
		}
		if len(articles) != 1 || articles[0].Content != wantContent {
			t.Fatalf("%s: unexpected articles: %#v", selector, articles)
		}
	}
}

func TestRefFromURLRoutesKnownHosts(t *testing.T) {
	tests := map[string]sourcedomain.Kind{
		"https://note.com/user/n/n1":         sourcedomain.KindNote,
		"https://zenn.dev/user/articles/abc": sourcedomain.KindZenn,
		"https://qiita.com/user/items/abc":   sourcedomain.KindQiita,
		"https://example.com/rss.xml":        sourcedomain.KindRSS,
		"https://example.com/blog/post":      sourcedomain.KindHTML,
	}
	for rawURL, want := range tests {
		if got := RefFromURL(rawURL).Kind; got != want {
			t.Fatalf("%s kind = %s, want %s", rawURL, got, want)
		}
	}
}

var testFetchedAt = parseFeedTime("2026-05-02T00:00:00+09:00")

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
	return t.next.RoundTrip(clone)
}
