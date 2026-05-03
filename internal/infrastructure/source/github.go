package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

// GitHubMarkdownFetcher reads Markdown files from public GitHub repositories.
// It is used for site-backed blogs such as corsweb2024 where the repository is
// the canonical source for frontmatter and Markdown body.
type GitHubMarkdownFetcher struct {
	client *http.Client
}

// NewGitHubMarkdownFetcher creates a GitHub Contents API fetcher.
func NewGitHubMarkdownFetcher(client *http.Client) *GitHubMarkdownFetcher {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &GitHubMarkdownFetcher{client: client}
}

// FetchList fetches Markdown files from a repository directory.
func (f *GitHubMarkdownFetcher) FetchList(ctx context.Context, ref sourcedomain.Ref, limit int) ([]sourcedomain.ArticleSnapshot, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	repoRef, err := parseGitHubRef(ref)
	if err != nil {
		return nil, err
	}
	entries, err := f.listContents(ctx, repoRef)
	if err != nil {
		return nil, err
	}
	snapshots := make([]sourcedomain.ArticleSnapshot, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if entry.Type != "file" || !isMarkdownPath(entry.Path) {
			continue
		}
		snapshot, err := f.snapshotFromDownload(ctx, repoRef, entry)
		if err != nil {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		left := snapshots[i].PublishedAt
		right := snapshots[j].PublishedAt
		if !left.IsZero() || !right.IsZero() {
			return left.After(right)
		}
		return snapshots[i].Title < snapshots[j].Title
	})
	if len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}
	return snapshots, nil
}

// FetchArticle fetches one Markdown file by GitHub URL or github: ref.
func (f *GitHubMarkdownFetcher) FetchArticle(ctx context.Context, ref sourcedomain.Ref) (*sourcedomain.ArticleSnapshot, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	repoRef, err := parseGitHubRef(ref)
	if err != nil {
		return nil, err
	}
	if !isMarkdownPath(repoRef.Path) {
		return nil, fmt.Errorf("github article path must be markdown: %s", repoRef.Path)
	}
	entry := githubContentEntry{
		Name:        path.Base(repoRef.Path),
		Path:        repoRef.Path,
		Type:        "file",
		DownloadURL: rawGitHubURL(repoRef),
		HTMLURL:     githubBlobURL(repoRef),
	}
	snapshot, err := f.snapshotFromDownload(ctx, repoRef, entry)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (f *GitHubMarkdownFetcher) listContents(ctx context.Context, ref githubRef) ([]githubContentEntry, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		strings.TrimLeft(ref.Path, "/"),
		url.QueryEscape(ref.Branch),
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create github contents request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch github contents: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch github contents: unexpected status %s", response.Status)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read github contents: %w", err)
	}
	var entries []githubContentEntry
	if err := json.Unmarshal(body, &entries); err == nil {
		return entries, nil
	}
	var entry githubContentEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, fmt.Errorf("decode github contents: %w", err)
	}
	return []githubContentEntry{entry}, nil
}

func (f *GitHubMarkdownFetcher) snapshotFromDownload(ctx context.Context, repoRef githubRef, entry githubContentEntry) (sourcedomain.ArticleSnapshot, error) {
	downloadURL := strings.TrimSpace(entry.DownloadURL)
	if downloadURL == "" {
		downloadURL = rawGitHubURL(githubRef{Owner: repoRef.Owner, Repo: repoRef.Repo, Branch: repoRef.Branch, Path: entry.Path})
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return sourcedomain.ArticleSnapshot{}, fmt.Errorf("create github raw request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/markdown, text/plain;q=0.9, */*;q=0.8")
	response, err := f.client.Do(request)
	if err != nil {
		return sourcedomain.ArticleSnapshot{}, fmt.Errorf("fetch github raw: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return sourcedomain.ArticleSnapshot{}, fmt.Errorf("fetch github raw: unexpected status %s", response.Status)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return sourcedomain.ArticleSnapshot{}, fmt.Errorf("read github raw: %w", err)
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		return sourcedomain.ArticleSnapshot{}, fmt.Errorf("github markdown was empty: %s", entry.Path)
	}
	metadata, markdown := splitFrontmatter(content)
	title := firstNonEmpty(metadata["title"], firstMarkdownHeading(markdown), strings.TrimSuffix(path.Base(entry.Path), path.Ext(entry.Path)))
	publishedAt := parseFeedTime(metadata["pubDate"])
	htmlURL := firstNonEmpty(entry.HTMLURL, githubBlobURL(githubRef{Owner: repoRef.Owner, Repo: repoRef.Repo, Branch: repoRef.Branch, Path: entry.Path}))
	return sourcedomain.ArticleSnapshot{
		ID:          entry.Path,
		Kind:        sourcedomain.KindGitHub,
		URL:         htmlURL,
		Title:       normalizeWhitespace(unquoteYAMLScalar(title)),
		Content:     content,
		PublishedAt: publishedAt,
		UpdatedAt:   publishedAt,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

func parseGitHubRef(ref sourcedomain.Ref) (githubRef, error) {
	if strings.TrimSpace(ref.URL) != "" {
		return parseGitHubURL(ref.URL)
	}
	value := strings.Trim(strings.TrimSpace(ref.Ref), "/")
	parts := strings.Split(value, "/")
	if len(parts) < 3 {
		return githubRef{}, fmt.Errorf("github ref must be owner/repo/path")
	}
	branch := "main"
	repoPathParts := parts[2:]
	if repo, branchValue, ok := strings.Cut(parts[1], "@"); ok {
		parts[1] = repo
		branch = branchValue
	}
	return githubRef{Owner: parts[0], Repo: parts[1], Branch: branch, Path: strings.Join(repoPathParts, "/")}, nil
}

func parseGitHubURL(rawURL string) (githubRef, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return githubRef{}, fmt.Errorf("parse github url: %w", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		if len(parts) < 5 || (parts[2] != "blob" && parts[2] != "tree") {
			return githubRef{}, fmt.Errorf("github url must be /owner/repo/blob-or-tree/branch/path")
		}
		return githubRef{Owner: parts[0], Repo: parts[1], Branch: parts[3], Path: strings.Join(parts[4:], "/")}, nil
	case host == "raw.githubusercontent.com":
		if len(parts) < 4 {
			return githubRef{}, fmt.Errorf("raw github url must be /owner/repo/branch/path")
		}
		return githubRef{Owner: parts[0], Repo: parts[1], Branch: parts[2], Path: strings.Join(parts[3:], "/")}, nil
	default:
		return githubRef{}, fmt.Errorf("unsupported github host %s", parsed.Hostname())
	}
}

func rawGitHubURL(ref githubRef) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", ref.Owner, ref.Repo, ref.Branch, strings.TrimLeft(ref.Path, "/"))
}

func githubBlobURL(ref githubRef) string {
	return fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", ref.Owner, ref.Repo, ref.Branch, strings.TrimLeft(ref.Path, "/"))
}

func isMarkdownPath(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}

func splitFrontmatter(content string) (map[string]string, string) {
	metadata := map[string]string{}
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return metadata, content
	}
	rest := strings.TrimPrefix(normalized, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return metadata, content
	}
	frontmatter := rest[:end]
	body := strings.TrimSpace(rest[end+len("\n---"):])
	for _, line := range strings.Split(frontmatter, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		metadata[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return metadata, body
}

func unquoteYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func firstMarkdownHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

type githubRef struct {
	Owner  string
	Repo   string
	Path   string
	Branch string
}

type githubContentEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
	HTMLURL     string `json:"html_url"`
}
