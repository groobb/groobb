package worker

// appliedRiverMigrationVersion is the River (the background job queue) schema
// migration version this project's own migrations are expected to match. A
// migration that follows River records the version it reaches in the
// river_migration tracking table. The constant anchors the drift-detection test
// in this package: when River's Go library ships a migration version beyond
// this number, that test fails and prompts a follow-up so the schema never
// silently lags behind the linked library.
//
// To advance it, run River's migrator against a scratch SQLite database with
// the new library version and read back the DDL SQLite stores in sqlite_master,
// add that as a new migration which appends
// `INSERT INTO river_migration (line, version) VALUES ('main', N);` (DELETE on
// down), then bump this constant to N.
//
// [Ja] appliedRiverMigrationVersion は、本プロジェクト自身のマイグレーションが一致して
// いるべき River (バックグラウンドジョブキュー) スキーママイグレーションのバージョン。
// River に追随するマイグレーションは、到達したバージョンを追跡テーブル river_migration
// に記録する。この定数は本パッケージのドリフト検知テストの基準値であり、River の Go
// ライブラリがこの番号を超えるマイグレーションバージョンを提供すると、そのテストが失敗
// して追随を促し、スキーマがリンク済みライブラリから静かに遅れることを防ぐ。
//
// 更新するときは、新しいライブラリバージョンで River のマイグレータを使い捨ての SQLite
// データベースに対して実行し、SQLite が sqlite_master に保持している DDL を読み出す。
// それを、末尾に `INSERT INTO river_migration (line, version) VALUES ('main', N);` を
// 追記する (down では DELETE) 新しいマイグレーションとして追加し、この定数を N に更新する。
const appliedRiverMigrationVersion = 7
