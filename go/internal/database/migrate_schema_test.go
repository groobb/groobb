package database_test

import (
	"context"
	"testing"
)

// TestMigrate_RestrictsCommunitiesToOneRow verifies that the singleton
// community constraint rejects a second row.
//
// [Ja] TestMigrate_RestrictsCommunitiesToOneRow は、コミュニティを単一行に保つ制約が
// 2 行目を拒否することを検証します。
func TestMigrate_RestrictsCommunitiesToOneRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	result, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Groobb")
	if err != nil {
		t.Fatalf("failed to insert the community: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read the community id: %v", err)
	}
	if id != 1 {
		t.Errorf("the first community id = %d, want 1", id)
	}

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (name) VALUES (?)", "Other"); err == nil {
		t.Error("inserting a second community should fail, but it succeeded")
	}
}

// TestMigrate_ListColumnsRequireJSONArrays verifies that list-valued columns
// accept empty and string arrays while rejecting every non-array JSON type.
//
// [Ja] TestMigrate_ListColumnsRequireJSONArrays は、リストを値に取る列が空配列と
// 文字列配列を受理し、配列以外の各 JSON 型を拒否することを検証します。
func TestMigrate_ListColumnsRequireJSONArrays(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	result, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	)
	if err != nil {
		t.Fatalf("failed to insert a user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read the user id: %v", err)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_two_factor_auths (user_id, secret) VALUES (?, ?)",
		userID, "secret",
	); err != nil {
		t.Fatalf("failed to insert two-factor authentication settings: %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO roles (name) VALUES (?)", "member"); err != nil {
		t.Fatalf("failed to insert a role: %v", err)
	}

	var recoveryCodes string
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT recovery_codes FROM user_two_factor_auths WHERE user_id = ?",
		userID,
	).Scan(&recoveryCodes); err != nil {
		t.Fatalf("failed to read the default recovery codes: %v", err)
	}
	if recoveryCodes != "[]" {
		t.Errorf("default recovery_codes = %q, want []", recoveryCodes)
	}

	var scopes string
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT scopes FROM roles WHERE name = ?",
		"member",
	).Scan(&scopes); err != nil {
		t.Fatalf("failed to read the default scopes: %v", err)
	}
	if scopes != "[]" {
		t.Errorf("default scopes = %q, want []", scopes)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"UPDATE user_two_factor_auths SET recovery_codes = ? WHERE user_id = ?",
		"[\"recovery-code\"]", userID,
	); err != nil {
		t.Fatalf("updating recovery_codes to a string array failed: %v", err)
	}
	if _, err := db.Writer.ExecContext(
		ctx,
		"UPDATE roles SET scopes = ? WHERE name = ?",
		"[\"read\"]", "member",
	); err != nil {
		t.Fatalf("updating scopes to a string array failed: %v", err)
	}

	invalidValues := []struct {
		name  string
		value string
	}{
		{name: "object", value: "{}"},
		{name: "null", value: "null"},
		{name: "number", value: "1"},
		{name: "string", value: "\"scope\""},
	}

	for _, tt := range invalidValues {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Writer.ExecContext(
				ctx,
				"UPDATE user_two_factor_auths SET recovery_codes = ? WHERE user_id = ?",
				tt.value, userID,
			); err == nil {
				t.Errorf("setting recovery_codes to %s should fail, but it succeeded", tt.value)
			}

			if _, err := db.Writer.ExecContext(
				ctx,
				"UPDATE roles SET scopes = ? WHERE name = ?",
				tt.value, "member",
			); err == nil {
				t.Errorf("setting scopes to %s should fail, but it succeeded", tt.value)
			}
		})
	}
}

// TestMigrate_EnforcesUserPasswordForeignKey verifies that a representative
// user child table rejects orphan rows and cascades deletion of its parent.
//
// [Ja] TestMigrate_EnforcesUserPasswordForeignKey は、代表的な users の子テーブルが
// 孤立行を拒否し、親行の削除を子行へカスケードすることを検証します。
func TestMigrate_EnforcesUserPasswordForeignKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := migratedTestDB(t)

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_passwords (user_id, password_digest) VALUES (?, ?)",
		999, "digest",
	); err == nil {
		t.Error("inserting a password for a missing user should fail, but it succeeded")
	}

	result, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)",
		"user@example.com", "user", "ja", "Asia/Tokyo",
	)
	if err != nil {
		t.Fatalf("failed to insert a user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to read the user id: %v", err)
	}

	if _, err := db.Writer.ExecContext(
		ctx,
		"INSERT INTO user_passwords (user_id, password_digest) VALUES (?, ?)",
		userID, "digest",
	); err != nil {
		t.Fatalf("failed to insert a password: %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID); err != nil {
		t.Fatalf("failed to delete the user: %v", err)
	}

	var passwordCount int
	if err := db.Reader.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM user_passwords WHERE user_id = ?",
		userID,
	).Scan(&passwordCount); err != nil {
		t.Fatalf("failed to count passwords after deleting the user: %v", err)
	}
	if passwordCount != 0 {
		t.Errorf("password count after deleting the user = %d, want 0", passwordCount)
	}
}
