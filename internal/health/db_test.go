package health

import (
"database/sql"
"testing"
)

// TestDBChecker_Creation tests that the DB checker is created correctly.
func TestDBChecker_Creation(t *testing.T) {
// Create a mock DB connection (won't actually connect)
db := &sql.DB{}

checker := NewDBChecker(db)
if checker == nil {
t.Fatal("expected checker to be non-nil")
}

if checker.db != db {
t.Error("expected checker db to match provided db")
}
}
