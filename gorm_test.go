package turso_go_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/tursodatabase/turso-go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Database struct {
	ID               uint `gorm:"primaryKey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
	Hostname         string         `gorm:"unique;not null"`
	Namespace        string
	Address          string
	PrimaryAddress   string
	CloudClusterName string
	Local            bool
	AllowedIPs       string
}

func openGormDB(t *testing.T) *gorm.DB {
	t.Helper()

	sqlDB, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sql connection: %v", err)
	}
	db, err := gorm.Open(sqlite.Dialector{
		Conn: sqlDB,
	}, &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm connection: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	return db
}

func TestGormBasicOperations(t *testing.T) {
	db := openGormDB(t)

	// Create table
	err := db.Debug().AutoMigrate(&Database{})
	if err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}

	record := Database{
		Hostname:         "test.local",
		Namespace:        "ns-test",
		Address:          "http://test:8080",
		CloudClusterName: "cluster-1",
	}

	result := db.Debug().Create(&record)
	if result.Error != nil {
		t.Fatalf("create failed: %v", result.Error)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("expected 1 row affected, got %d", result.RowsAffected)
	}
	if record.ID == 0 {
		t.Fatal("expected ID to be set after create")
	}

	var found Database
	err = db.Debug().Where("hostname = ?", "test.local").First(&found).Error
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if found.Hostname != "test.local" {
		t.Fatalf("unexpected hostname: %s", found.Hostname)
	}
}

func TestGormUpsert(t *testing.T) {
	db := openGormDB(t)
	err := db.AutoMigrate(&Database{})
	if err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}

	record := Database{
		Hostname:  "upsert-test.local",
		Namespace: "ns-1",
		Address:   "http://addr1",
	}

	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}},
		UpdateAll: true,
	}).Create(&record).Error

	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	originalID := record.ID
	if originalID == 0 {
		t.Fatal("expected ID to be set after upsert")
	}

	record2 := Database{
		Hostname:  "upsert-test.local",
		Namespace: "ns-2",
		Address:   "http://addr2",
	}

	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hostname"}},
		UpdateAll: true,
	}).Create(&record2).Error

	if err != nil {
		t.Fatalf("second upsert (update) failed: %v", err)
	}

	var count int64
	db.Model(&Database{}).Where("hostname = ?", "upsert-test.local").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 record, got %d", count)
	}

	var updated Database
	db.Where("hostname = ?", "upsert-test.local").First(&updated)
	if updated.Namespace != "ns-2" {
		t.Fatalf("expected namespace to be updated to ns-2, got %s", updated.Namespace)
	}
}

func TestGormUpsertWithReturning(t *testing.T) {
	db := openGormDB(t)
	err := db.AutoMigrate(&Database{})
	if err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}

	t.Run("RawSQLReturning", func(t *testing.T) {
		sqlDB, _ := db.DB()

		const query = `
		INSERT INTO databases (created_at, updated_at, hostname, namespace, address, primary_address, cloud_cluster_name, local, allowed_ips)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (hostname) DO UPDATE SET
			updated_at = excluded.updated_at,
			namespace = excluded.namespace
		RETURNING id`

		stmt, err := sqlDB.Prepare(query)
		if err != nil {
			t.Fatalf("prepare failed: %v", err)
		}
		defer stmt.Close()

		now := time.Now()
		var returnedID int64
		err = stmt.QueryRow(
			now, now, "raw-test.local", "ns-raw",
			"http://raw", "", "cluster-raw", false, "",
		).Scan(&returnedID)

		if err != nil {
			t.Fatalf("raw upsert with returning failed: %v", err)
		}
		if returnedID == 0 {
			t.Fatal("expected non-zero ID from RETURNING")
		}
		t.Logf("Raw SQL RETURNING worked, ID: %d", returnedID)
	})

	t.Run("BatchUpsert", func(t *testing.T) {
		records := []Database{
			{Hostname: "batch1.local", Namespace: "ns1"},
			{Hostname: "batch2.local", Namespace: "ns2"},
			{Hostname: "batch3.local", Namespace: "ns3"},
		}

		err := db.Debug().Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}},
			UpdateAll: true,
		}).Create(&records).Error
		if err != nil {
			t.Fatalf("batch upsert failed: %v", err)
		}

		for i, r := range records {
			if r.ID == 0 {
				t.Errorf("record %d has zero ID after batch upsert", i)
			}
		}
	})
}

func TestGormSoftDelete(t *testing.T) {
	db := openGormDB(t)
	err := db.AutoMigrate(&Database{})
	if err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}
	testData := []Database{
		{Hostname: "soft1.local", Namespace: "ns1"},
		{Hostname: "soft2.local", Namespace: "ns2"},
		{Hostname: "soft3.local", Namespace: "ns3"},
	}
	err = db.Create(&testData).Error
	if err != nil {
		t.Fatalf("create test data failed: %v", err)
	}
	// delete a row
	err = db.Where("hostname = ?", "soft2.local").Delete(&Database{}).Error
	if err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}
	// select non-deleted rows
	var rows []Database
	err = db.Find(&rows).Error
	if err != nil {
		t.Fatalf("find non-deleted failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 non-deleted rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r.Hostname == "soft2.local" {
			t.Fatalf("soft-deleted record still found in normal query")
		}
	}
}

func TestGormComplexQueries(t *testing.T) {
	db := openGormDB(t)
	err := db.AutoMigrate(&Database{})
	if err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}

	testData := []Database{
		{Hostname: "host1.local", Namespace: "ns1", UpdatedAt: time.Now().Add(-2 * time.Hour)},
		{Hostname: "host2.local", Namespace: "ns2", UpdatedAt: time.Now().Add(-1 * time.Hour)},
		{Hostname: "host3.local", Namespace: "ns3", UpdatedAt: time.Now()},
	}

	for _, d := range testData {
		db.Create(&d)
	}
	db.Where("hostname = ?", "host2.local").Delete(&Database{})

	t.Run("GetNonDeleted", func(t *testing.T) {
		rows, err := db.Debug().Model(&Database{}).Where("deleted_at IS NULL").Rows()
		if err != nil {
			t.Fatalf("get non-deleted failed: %v", err)
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var database Database
			err := db.ScanRows(rows, &database)
			if err != nil {
				t.Fatalf("scan rows failed: %v", err)
			}
			count++
		}
		// Should find host1 and host3
		if count != 2 {
			t.Fatalf("expected 2 non-deleted records, got %d", count)
		}
	})

	t.Run("UnscopedQuery", func(t *testing.T) {
		var count int64
		db.Unscoped().Model(&Database{}).Count(&count)
		if count != 3 {
			t.Fatalf("expected 3 total records (including soft-deleted), got %d", count)
		}
	})
}

func TestGormColumnCountIssue(t *testing.T) {
	db := openGormDB(t)
	err := db.AutoMigrate(&Database{})
	if err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}

	t.Run("UpsertReturningColumnCount", func(t *testing.T) {
		record := Database{
			Hostname: "column-test.local",
		}

		err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}},
			UpdateAll: true,
		}).Create(&record).Error

		if err != nil {
			t.Fatalf("first upsert failed: %v", err)
		}

		record2 := Database{
			Hostname:         "column-test.local",
			Namespace:        "ns-test",
			Address:          "http://test",
			PrimaryAddress:   "http://primary",
			CloudClusterName: "cluster",
			Local:            true,
			AllowedIPs:       "192.168.1.0/24",
		}

		err = db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hostname"}},
			UpdateAll: true,
		}).Create(&record2).Error

		if err != nil {
			// This is where we expect to catch the column mismatch error
			t.Logf("Caught potential column mismatch error: %v", err)
			t.Fatalf("second upsert failed: %v", err)
		}
	})

	t.Run("ExplicitReturningClause", func(t *testing.T) {
		sqlDB, _ := db.DB()

		testCases := []struct {
			name      string
			returning string
		}{
			{"returning_id", "RETURNING id"},
			{"returning_star", "RETURNING *"},
			{"returning_multiple", "RETURNING id, updated_at, hostname"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				query := `
				INSERT INTO databases (hostname, namespace)
				VALUES (?, ?)
				ON CONFLICT (hostname) DO UPDATE SET
					namespace = excluded.namespace
				` + tc.returning

				rows, err := sqlDB.Query(query, tc.name+".local", "ns-test")
				if err != nil {
					t.Fatalf("%s query failed: %v", tc.name, err)
				}
				defer rows.Close()

				cols, err := rows.Columns()
				if err != nil {
					t.Fatalf("get columns failed: %v", err)
				}
				t.Logf("%s returned %d columns: %v", tc.name, len(cols), cols)
			})
		}
	})
}

func TestGormLastMethod(t *testing.T) {
	db := openGormDB(t)
	err := db.AutoMigrate(&Database{})
	if err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}

	for i := 1; i <= 3; i++ {
		db.Create(&Database{
			Hostname:  fmt.Sprintf("last-test-%d.local", i),
			Namespace: fmt.Sprintf("ns-%d", i),
		})
		time.Sleep(10 * time.Millisecond)
	}
	var database Database
	err = db.Where(Database{Hostname: "last-test-2.local"}).Last(&database).Error
	if err != nil {
		t.Fatalf("Last() failed: %v", err)
	}

	if database.Hostname != "last-test-2.local" {
		t.Fatalf("unexpected hostname from Last(): %s", database.Hostname)
	}
}

func TestGormPartialIndexes(t *testing.T) {
	db := openGormDB(t)

	sqlDB, _ := db.DB()

	_, err := sqlDB.Exec(`
		CREATE TABLE partial_index_test (
			id INTEGER PRIMARY KEY,
			status TEXT,
			priority INTEGER,
			deleted_at TIMESTAMP,
			email TEXT,
			active BOOLEAN
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Test creating partial indexes
	t.Run("CreatePartialIndexes", func(t *testing.T) {
		_, err := sqlDB.Exec(`
			CREATE UNIQUE INDEX idx_active_email 
			ON partial_index_test(email) 
			WHERE active = 1
		`)
		if err != nil {
			t.Fatalf("failed to create partial unique index: %v", err)
		}

		_, err = sqlDB.Exec(`
			CREATE INDEX idx_high_priority 
			ON partial_index_test(priority, status) 
			WHERE priority > 5 AND status != 'archived'
		`)
		if err != nil {
			t.Fatalf("failed to create partial index with complex WHERE: %v", err)
		}

		_, err = sqlDB.Exec(`
			CREATE INDEX idx_not_deleted 
			ON partial_index_test(status) 
			WHERE deleted_at IS NULL
		`)
		if err != nil {
			t.Fatalf("failed to create partial index with IS NULL: %v", err)
		}
	})

	t.Run("PartialUniqueConstraint", func(t *testing.T) {
		// Insert active user with email
		_, err := sqlDB.Exec(`
			INSERT INTO partial_index_test (email, active, status, priority) 
			VALUES (?, ?, ?, ?)
		`, "user@example.com", true, "active", 3)
		if err != nil {
			t.Fatalf("failed to insert first active user: %v", err)
		}

		// Should succeed - same email but inactive user
		_, err = sqlDB.Exec(`
			INSERT INTO partial_index_test (email, active, status, priority) 
			VALUES (?, ?, ?, ?)
		`, "user@example.com", false, "inactive", 2)
		if err != nil {
			t.Fatalf("failed to insert inactive user with same email: %v", err)
		}

		// Should fail - duplicate email for active user
		_, err = sqlDB.Exec(`
			INSERT INTO partial_index_test (email, active, status, priority) 
			VALUES (?, ?, ?, ?)
		`, "user@example.com", true, "active", 5)
		if err == nil {
			t.Fatal("expected unique constraint violation for duplicate active email")
		}
	})

	t.Run("ComplexWhereClause", func(t *testing.T) {
		_, err := sqlDB.Exec(`
			CREATE INDEX IF NOT EXISTS idx_complex 
			ON partial_index_test(id) 
			WHERE (priority BETWEEN 3 AND 7) AND status IN ('active', 'pending')
		`)
		if err != nil {
			t.Fatalf("failed to create complex partial index: %v", err)
		}

	})
}
