package note

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Fetcher はNote記事取得サービス
type Fetcher struct {
	client *http.Client
}

// NewFetcher は新しいFetcherを作成
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{},
	}
}

// FetchUserLatestArticles は指定されたユーザー名から最新の記事を取得
func (f *Fetcher) FetchUserLatestArticles(username string, count int) ([]string, error) {
	if count <= 0 {
		count = 3 // デフォルトは3記事
	}

	// ユーザーページのURL
	userURL := fmt.Sprintf("https://note.com/%s", username)

	// リクエストの作成
	req, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// User-Agentの設定
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// リクエストの実行
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user page: %w", err)
	}
	defer resp.Body.Close()

	// レスポンスの検証
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch user page: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// HTMLの解析
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// 記事URLの取得
	var articleURLs []string
	doc.Find("a.o-noteContentLink").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists && i < count {
			articleURLs = append(articleURLs, "https://note.com"+href)
		}
	})

	// 記事の取得
	var articles []string
	for _, url := range articleURLs {
		article, err := f.FetchArticle(url)
		if err != nil {
			fmt.Printf("Warning: Failed to fetch article %s: %v\n", url, err)
			continue
		}
		articles = append(articles, article)
	}

	return articles, nil
}

// FetchArticle は指定されたURLからNote記事を取得
func (f *Fetcher) FetchArticle(url string) (string, error) {
	// URLの検証
	if !strings.HasPrefix(url, "https://note.com/") {
		return "", fmt.Errorf("invalid note URL: %s", url)
	}

	// リクエストの作成
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// User-Agentの設定
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// リクエストの実行
	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch article: %w", err)
	}
	defer resp.Body.Close()

	// レスポンスの検証
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch article: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// HTMLの解析
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// 記事本文の取得
	var content strings.Builder
	doc.Find("div.o-noteContentText").Each(func(i int, s *goquery.Selection) {
		content.WriteString(s.Text())
		content.WriteString("\n\n")
	})

	// 記事タイトルの取得
	title := doc.Find("h1.o-noteContentTitle").Text()
	if title == "" {
		title = "無題の記事"
	}

	// 記事の整形
	article := fmt.Sprintf("# %s\n\n%s", title, content.String())
	return article, nil
}
