package note

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Article は記事の情報を表す構造体
type Article struct {
	URL     string
	Title   string
	Content string
}

// Fetcher はNote APIから記事を取得するサービス
type Fetcher struct {
	client *http.Client
}

// NewFetcher は新しいFetcherを作成
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchUserLatestArticles は指定されたユーザーの最新記事を取得
func (f *Fetcher) FetchUserLatestArticles(userID string, limit int) ([]Article, error) {
	// Note APIのエンドポイント
	url := fmt.Sprintf("https://note.com/api/v2/creators/%s/contents?kind=note&page=1", userID)
	log.Printf("記事一覧を取得: %s", url)

	// APIリクエスト
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエストの作成に失敗: %w", err)
	}

	// User-Agentヘッダーを追加
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// レスポンスの取得
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("APIリクエストに失敗: %w", err)
	}
	defer resp.Body.Close()

	// ステータスコードの確認
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("APIエラー: %s", resp.Status)
	}

	// レスポンスの解析
	var response struct {
		Data struct {
			Contents []struct {
				Name      string `json:"name"`
				NoteURL   string `json:"noteUrl"`
				Status    string `json:"status"`
				PublishAt string `json:"publishAt"`
			} `json:"contents"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("レスポンスの解析に失敗: %w", err)
	}

	log.Printf("取得した記事数: %d", len(response.Data.Contents))

	// 公開済みの記事のみを抽出
	var articles []Article
	for i, content := range response.Data.Contents {
		log.Printf("記事 %d: %s (ステータス: %s)", i+1, content.Name, content.Status)
		if content.Status == "published" {
			// 記事の内容を取得
			article, err := f.FetchArticle(content.NoteURL)
			if err != nil {
				log.Printf("記事の取得に失敗: %v", err)
				continue // エラーが発生した場合は次の記事へ
			}
			log.Printf("記事の内容を取得: %s (長さ: %d文字)", article.Title, len(article.Content))
			articles = append(articles, *article)
			if len(articles) >= limit {
				break
			}
		}
	}

	log.Printf("有効な記事数: %d", len(articles))
	return articles, nil
}

// FetchArticle は指定されたURLの記事を取得
func (f *Fetcher) FetchArticle(url string) (*Article, error) {
	// URLからnote IDを抽出
	noteID := extractNoteID(url)
	if noteID == "" {
		return nil, fmt.Errorf("無効なnote URL: %s", url)
	}

	// Note APIの記事詳細エンドポイント
	apiURL := fmt.Sprintf("https://note.com/api/v3/notes/%s", noteID)
	log.Printf("記事の詳細を取得: %s (note ID: %s)", apiURL, noteID)

	// リクエストの作成
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエストの作成に失敗: %w", err)
	}

	// User-Agentヘッダーを追加
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// レスポンスの取得
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("記事の取得に失敗: %w", err)
	}
	defer resp.Body.Close()

	// ステータスコードの確認
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("記事の取得に失敗: %s", resp.Status)
	}

	// レスポンスの解析
	var response struct {
		Data struct {
			Name string `json:"name"`
			Body string `json:"body"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("レスポンスの解析に失敗: %w", err)
	}

	article := &Article{
		URL:     url,
		Title:   response.Data.Name,
		Content: response.Data.Body,
	}

	log.Printf("記事の詳細を取得完了: %s (長さ: %d文字)", article.Title, len(article.Content))
	return article, nil
}

// extractNoteID はnote URLからnote IDを抽出
func extractNoteID(url string) string {
	// URLの形式: https://note.com/username/n/noteID
	parts := strings.Split(url, "/")
	if len(parts) >= 5 {
		// URLの最後の部分がnote ID
		noteID := parts[len(parts)-1]
		log.Printf("URLから抽出したnote ID: %s (元のURL: %s)", noteID, url)
		return noteID
	}
	log.Printf("無効なURL形式: %s", url)
	return ""
}

// FetchArticlesByKeyword はキーワードで記事を検索
func (f *Fetcher) FetchArticlesByKeyword(keyword string, limit int) ([]Article, error) {
	// Note APIの検索エンドポイント
	url := fmt.Sprintf("https://api.note.com/v1/search/articles?q=%s&limit=%d", keyword, limit)

	// APIリクエスト
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエストの作成に失敗: %w", err)
	}

	// レスポンスの取得
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("APIリクエストに失敗: %w", err)
	}
	defer resp.Body.Close()

	// ステータスコードの確認
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("APIエラー: %s", resp.Status)
	}

	// レスポンスの解析
	var articles []Article
	if err := json.NewDecoder(resp.Body).Decode(&articles); err != nil {
		return nil, fmt.Errorf("レスポンスの解析に失敗: %w", err)
	}

	return articles, nil
}

// FetchArticleContent は記事の本文を取得
func (f *Fetcher) FetchArticleContent(url string) (string, error) {
	article, err := f.FetchArticle(url)
	if err != nil {
		return "", err
	}
	return article.Content, nil
}

// FetchMultipleArticles は複数の記事を取得
func (f *Fetcher) FetchMultipleArticles(urls []string) ([]Article, error) {
	var articles []Article
	for i, url := range urls {
		log.Printf("記事 %d/%d を取得: %s", i+1, len(urls), url)
		article, err := f.FetchArticle(url)
		if err != nil {
			return nil, fmt.Errorf("記事の取得に失敗 (%s): %w", url, err)
		}
		articles = append(articles, *article)
	}
	log.Printf("全記事の取得完了: %d件", len(articles))
	return articles, nil
}
