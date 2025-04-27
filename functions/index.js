/**
 * Import function triggers from their respective submodules:
 *
 * const {onCall} = require("firebase-functions/v2/https");
 * const {onDocumentWritten} = require("firebase-functions/v2/firestore");
 *
 * See a full list of supported triggers at https://firebase.google.com/docs/functions
 */

const {onRequest} = require("firebase-functions/v2/https");
const logger = require("firebase-functions/logger");
const {GoogleGenerativeAI} = require("@google/generative-ai");
const functions = require("firebase-functions");

// Create and deploy your first functions
// https://firebase.google.com/docs/functions/get-started

// exports.helloWorld = onRequest((request, response) => {
//   logger.info("Hello logs!", {structuredData: true});
//   response.send("Hello from Firebase!");
// });

// Gemini APIの初期化
const genAI = new GoogleGenerativeAI(
    process.env.GEMINI_API_KEY || functions.config().gemini.api_key,
);

// モデルの設定を更新
const model = genAI.getGenerativeModel({ 
    model: "gemini-2.5-pro-preview-03-25",
    generationConfig: {
        temperature: 0.7,
        topP: 0.95,
        topK: 40,
        maxOutputTokens: 8192,
    },
});

exports.generateArticle = onRequest(async (request, response) => {
  try {
    // CORSの設定
    response.set("Access-Control-Allow-Origin", "*");
    response.set("Access-Control-Allow-Methods", "POST");
    response.set("Access-Control-Allow-Headers", "Content-Type");

    if (request.method === "OPTIONS") {
      response.status(204).send("");
      return;
    }

    if (request.method !== "POST") {
      response.status(405).send("Method Not Allowed");
      return;
    }

    const {
      noteUrl,
      username,
      keywords,
      theme,
      targetAudience,
      styleChoice,
      toneChoice,
      wordCount,
      articlePurpose,
      desiredContent,
    } = request.body;

    // プロンプトの構築
    const prompt = `
以下のパラメータに基づいて、Note記事の下書きを生成してください：

参照URL: ${noteUrl || "なし"}
ユーザー名: ${username || "なし"}
キーワード: ${keywords || "なし"}
テーマ: ${theme || "なし"}
読者層: ${targetAudience || "なし"}
文体: ${styleChoice || "なし"}
トーン: ${toneChoice || "なし"}
目標文字数: ${wordCount || "1000"}文字
記事の目的: ${articlePurpose || "なし"}
具体的な内容: ${desiredContent || "なし"}

マークダウン形式で出力してください。
`;

    // Gemini APIを使用して記事を生成
    const result = await model.generateContent(prompt);
    const generatedText = result.response.text();

    response.json({
      draft: generatedText,
    });
  } catch (error) {
    logger.error("Error generating article:", error);
    response.status(500).json({
      error: {
        message: "記事の生成中にエラーが発生しました",
        details: error.message,
      },
    });
  }
});
