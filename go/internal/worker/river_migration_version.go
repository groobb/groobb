package worker

// appliedRiverMigrationVersionは、本プロジェクト自身のマイグレーションが一致して
// いるべきRiver (バックグラウンドジョブキュー) スキーママイグレーションのバージョン。
// Riverに追随するマイグレーションは、到達したバージョンを追跡テーブルriver_migration
// に記録する。この定数は本パッケージのドリフト検知テストの基準値であり、RiverのGo
// ライブラリがこの番号を超えるマイグレーションバージョンを提供すると、そのテストが失敗
// して追随を促し、スキーマがリンク済みライブラリから静かに遅れることを防ぐ。
//
// 更新するときは、新しいライブラリバージョンでRiverのマイグレータを使い捨てのSQLite
// データベースに対して実行し、SQLiteがsqlite_masterに保持しているDDLを読み出す。
// それを、末尾に `INSERT INTO river_migration (line, version) VALUES ('main', N);` を
// 追記する (downではDELETE) 新しいマイグレーションとして追加し、この定数をNに更新する。
const appliedRiverMigrationVersion = 7
