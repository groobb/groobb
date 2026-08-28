<!-- last_synced: 2026-08-28 -->

# Groobb (グルーブ)

> [English](./README.md) | 日本語

自分の居場所を見つけよう

Groobb は開発中で、まだ一般公開していません。
セルフホスト用のバイナリとインストール手順も用意できていません。

[![Go CI](https://github.com/groobb/groobb/actions/workflows/go-ci.yml/badge.svg)](https://github.com/groobb/groobb/actions/workflows/go-ci.yml)

## Groobb について

Groobb は、自分のサーバで動かせる掲示板サービスです。
掲示板は複数作ることができ、その中にスレッドを立てて会話します。
インストールと運用の手間を小さく保ち、簡単にセルフホストできることを目指しています。

## 設計

データベースは SQLite だけを使います。
別のデータベースサーバを用意する必要はなく、データはファイル 1 つに収まります。

静的アセット・翻訳ファイル・マイグレーションはバイナリに同梱しています。
実行に必要なものがバイナリの中で完結するため、置いたディレクトリに依存せずに動きます。

## 関連リンク

- [コントリビューションについて](./CONTRIBUTING.ja.md)
- [セキュリティに関する報告](./SECURITY.ja.md)

## ライセンス

Groobb は [GNU Affero General Public License v3.0](./LICENSE) で公開しています。
