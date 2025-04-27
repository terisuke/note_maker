document.addEventListener('DOMContentLoaded', function() {
    // 要素の取得
    const noteUrlInput = document.getElementById('note-url');
    const usernameInput = document.getElementById('username');
    const keywordsInput = document.getElementById('keywords');
    const themeInput = document.getElementById('theme');
    const targetAudienceInput = document.getElementById('target-audience');
    const exclusionsInput = document.getElementById('exclusions');
    const styleChoiceSelect = document.getElementById('style-choice');
    const toneChoiceSelect = document.getElementById('tone-choice');
    const wordCountSelect = document.getElementById('word-count');
    const generateBtn = document.getElementById('generate-btn');
    const resultSection = document.getElementById('result-section');
    const markdownOutput = document.getElementById('markdown-output');
    const previewContent = document.getElementById('preview-content');
    const loadingDiv = document.getElementById('loading');
    const copyBtn = document.getElementById('copy-btn');
    const tabBtns = document.querySelectorAll('.tab-btn');
    const tabPanes = document.querySelectorAll('.tab-pane');
    const errorMessageArea = document.getElementById('error-message-area');

    // 記事生成ボタンクリック時の処理
    generateBtn.addEventListener('click', function() {
        clearErrorMessage();
        const noteUrl = noteUrlInput.value.trim();
        const username = usernameInput.value.trim();
        const keywords = keywordsInput.value.trim().split(',').map(k => k.trim()).filter(k => k !== '');
        const theme = themeInput.value.trim();
        const targetAudience = targetAudienceInput.value.trim();
        const exclusions = exclusionsInput.value.trim();
        const styleChoice = styleChoiceSelect.value;
        const toneChoice = toneChoiceSelect.value;
        const wordCount = parseInt(wordCountSelect.value);

        // 入力検証
        if (!noteUrl && !username) {
            showErrorMessage('Note記事URLまたはユーザー名を入力してください');
            return;
        }

        if (noteUrl) {
            try {
                new URL(noteUrl);
                if (!noteUrl.includes('note.com')) {
                    showErrorMessage('有効なNote記事URLを入力してください');
                    return;
                }
            } catch (_) {
                showErrorMessage('有効なURL形式で入力してください');
                return;
            }
        }

        // APIリクエストデータ構築
        const requestData = {
            note_url: noteUrl,
            username: username,
            keywords: keywords,
            theme: theme,
            target_audience: targetAudience,
            exclusions: exclusions,
            style_choice: styleChoice,
            tone_choice: toneChoice,
            word_count: wordCount
        };

        // 記事の生成開始
        generateArticle(requestData);
    });

    // 記事の生成
    function generateArticle(requestData) {
        loadingDiv.classList.remove('hidden');
        resultSection.classList.add('hidden');
        generateBtn.disabled = true;

        fetch('/api/generate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(requestData)
        })
        .then(async response => {
            if (!response.ok) {
                const errorData = await response.json().catch(() => ({
                    error: { message: `サーバーエラーが発生しました (Status: ${response.status})` }
                }));
                const errorMessage = errorData.error?.message || `サーバーエラーが発生しました (Status: ${response.status})`;
                throw new Error(errorMessage);
            }
            return response.json();
        })
        .then(data => {
            if (data.draft) {
                displayGeneratedArticle(data.draft);
            } else {
                showErrorMessage('生成された記事が見つかりません');
            }
        })
        .catch(error => {
            console.error('Generation failed:', error);
            showErrorMessage(`記事の生成に失敗しました: ${error.message}`);
        })
        .finally(() => {
            loadingDiv.classList.add('hidden');
            generateBtn.disabled = false;
        });
    }

    // 生成された記事の表示
    function displayGeneratedArticle(markdown) {
        markdownOutput.value = markdown;
        previewContent.innerHTML = marked.parse(markdown);
        resultSection.classList.remove('hidden');
        setActiveTab('preview');
        resultSection.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }

    // エラーメッセージの表示
    function showErrorMessage(message) {
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-message';
        errorDiv.textContent = message;
        errorMessageArea.appendChild(errorDiv);
    }

    // エラーメッセージのクリア
    function clearErrorMessage() {
        errorMessageArea.innerHTML = '';
    }

    // タブ切り替え
    tabBtns.forEach(btn => {
        btn.addEventListener('click', function() {
            const tabId = this.getAttribute('data-tab');
            setActiveTab(tabId);
        });
    });

    // 特定のタブをアクティブにする関数
    function setActiveTab(tabId) {
        tabBtns.forEach(b => b.classList.remove('active'));
        document.querySelector(`.tab-btn[data-tab="${tabId}"]`).classList.add('active');

        tabPanes.forEach(pane => pane.classList.remove('active'));
        document.getElementById(`${tabId}-tab`).classList.add('active');
    }

    // コピーボタン
    copyBtn.addEventListener('click', function() {
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(markdownOutput.value).then(() => {
                showCopySuccessMessage(this);
            }).catch(err => {
                console.error('Clipboard copy failed:', err);
                fallbackCopyTextToClipboard(markdownOutput.value, this);
            });
        } else {
            fallbackCopyTextToClipboard(markdownOutput.value, this);
        }
    });

    function fallbackCopyTextToClipboard(text, buttonElement) {
        markdownOutput.select();
        try {
            const successful = document.execCommand('copy');
            if (successful) {
                showCopySuccessMessage(buttonElement);
            } else {
                console.error('Fallback copy command failed');
                alert('コピーに失敗しました。手動でコピーしてください。');
            }
        } catch (err) {
            console.error('Fallback copy error:', err);
            alert('コピーに失敗しました。手動でコピーしてください。');
        }
        window.getSelection().removeAllRanges();
    }

    function showCopySuccessMessage(buttonElement) {
        const originalText = buttonElement.textContent;
        buttonElement.textContent = 'コピーしました！';
        buttonElement.disabled = true;
        setTimeout(() => {
            buttonElement.textContent = originalText;
            buttonElement.disabled = false;
        }, 2000);
    }
}); 