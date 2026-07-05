// Entry point for Groobb's client-side JavaScript bundle, built by esbuild into
// static/js/main.js. Importing "basecoat-css/all" registers every Basecoat JS
// component and exposes the window.basecoat runtime, which auto-initializes
// components on DOMContentLoaded and watches the DOM for dynamically inserted
// ones. No interactive component is used yet, but wiring "all" now means adding
// one later needs no change here.
//
// [Ja] Groobb のクライアントサイド JavaScript バンドルのエントリポイント。esbuild が
// static/js/main.js にビルドする。"basecoat-css/all" を読み込むと Basecoat の全 JS
// コンポーネントが登録され、window.basecoat ランタイムが公開される。ランタイムは
// DOMContentLoaded でコンポーネントを自動初期化し、動的に挿入された DOM も監視する。
// interactive コンポーネントはまだ使わないが、今 "all" を配線しておけば後で足すときに
// ここの変更が不要になる。
import "basecoat-css/all";
