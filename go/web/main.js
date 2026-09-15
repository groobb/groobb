// GroobbのクライアントサイドJavaScriptバンドルのエントリポイント。esbuildが
// static/js/main.jsにビルドする。"basecoat-css/all" を読み込むとBasecoatの全JS
// コンポーネントが登録され、window.basecoatランタイムが公開される。ランタイムは
// DOMContentLoadedでコンポーネントを自動初期化し、動的に挿入されたDOMも監視する。
// フラッシュメッセージのtoastを初期化するため、Basecoatのランタイムを読み込む。
import "basecoat-css/all";
