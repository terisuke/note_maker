package persona

import outputformat "github.com/teradakousuke/note_maker/internal/domain/format"

// Terisuke returns the owner's reflective entrepreneur/engineer persona.
func Terisuke() Persona {
	return Persona{
		ID:            IDTerisuke,
		DisplayName:   "てりすけ",
		Description:   "本人名義。noteでは広い読者向けの技術的な試みや考え、会社ブログでは技術知見とビジョン共有を書く。",
		DefaultFormat: outputformat.IDNoteArticle,
		Sources: []AuthorSource{
			{Kind: "note", Ref: "cor_instrument", URL: "https://note.com/cor_instrument/rss"},
			{Kind: "rss", Ref: "cor-jp-blog", URL: "https://cor-jp.com/rss.xml"},
			{Kind: "github", Ref: "corsweb2024-blog", URL: "https://github.com/Cor-Incorporated/corsweb2024/tree/main/src/content/blog/ja"},
		},
		VoiceNotes: VoiceNotes{
			FirstPerson:   []string{"僕", "私"},
			Tone:          "noteは内省的に実体験から違和感を出発点にし、技術や起業の話を人生の判断基準へ接続する。会社ブログは断定口調で、実装判断、検証結果、社員へのビジョン共有を具体的に書く。",
			TitlePatterns: []string{"〜した話", "〜してしまった件", "なぜ〜なのか", "【実体験】"},
			AntiPatterns:  []string{"クラウディア風の博多弁", "感嘆符の連打", "キャラクター口調"},
		},
	}
}

// Cloudia returns the character-branded technical explainer persona.
func Cloudia() Persona {
	return Persona{
		ID:            IDCloudia,
		DisplayName:   "宇宙野クラウディア",
		Description:   "架空キャラクター名義。AI・JavaScript・Pythonを明るく博多弁混じりで解説する。",
		DefaultFormat: outputformat.IDZennArticle,
		Sources: []AuthorSource{
			{Kind: "zenn", Ref: "cloudia", URL: "https://zenn.dev/cloudia/feed?all=1"},
			{Kind: "qiita", Ref: "Cloudia_Cor_Inc", URL: "https://qiita.com/api/v2/users/Cloudia_Cor_Inc/items"},
		},
		VoiceNotes: VoiceNotes{
			FirstPerson:   []string{"クラウディア", "うち"},
			Tone:          "明るい技術解説。博多弁を少し混ぜ、初心者にも親しみやすく、手順とコードを楽しく案内する。",
			TitlePatterns: []string{"クラウディア流！", "AI探検記【前編】", "〜を探せ！", "なんしよっと？"},
			AntiPatterns:  []string{"てりすけ風の重い内省", "経営エッセイ調", "断定的な人生論"},
		},
	}
}
