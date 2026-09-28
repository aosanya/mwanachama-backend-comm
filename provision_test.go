package mwanachamacomm

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	"github.com/aosanya/mwanachama-backend-comm/gormstore"
)

func provisionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	return db
}

func TestProvisionCreatesEveryDeclaredTable(t *testing.T) {
	db := provisionTestDB(t)
	s := testSpec(t)

	if err := Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	for _, o := range s.Objects {
		table := s.TableFor(o)
		if !db.Migrator().HasTable(table) {
			t.Errorf("Provision left %s (role %q) uncreated", table, o.Role)
		}
	}
}

func TestProvisionIsIdempotent(t *testing.T) {
	db := provisionTestDB(t)
	s := testSpec(t)

	for i := 0; i < 3; i++ {
		if err := Provision(db, s); err != nil {
			t.Fatalf("Provision run %d: %v", i+1, err)
		}
	}
}

func TestProvisionMovesALegacyTableSetOntoTheDeclaredNames(t *testing.T) {
	db := provisionTestDB(t)
	s := testSpec(t)

	if err := gormstore.Migrate(db, gormstore.DefaultTableNames()); err != nil {
		t.Fatalf("seed the pre-spec table set: %v", err)
	}
	for _, legacy := range legacyTables {
		if !db.Migrator().HasTable(legacy) {
			t.Fatalf("seed did not create %s", legacy)
		}
	}

	if err := Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	for role, legacy := range legacyTables {
		o, ok := s.ByRole(role)
		if !ok {
			t.Fatalf("role %q fills nothing", role)
		}
		declared := s.TableFor(o)
		if !db.Migrator().HasTable(declared) {
			t.Errorf("%s was not moved onto %s", legacy, declared)
		}
		if db.Migrator().HasTable(legacy) {
			t.Errorf("%s still exists beside %s, so a read could reach either", legacy, declared)
		}
	}
}

func TestProvisionRenamesTheChapterAndMemberColumns(t *testing.T) {
	db := provisionTestDB(t)
	s := testSpec(t)

	if err := gormstore.Migrate(db, gormstore.DefaultTableNames()); err != nil {
		t.Fatalf("seed the pre-spec table set: %v", err)
	}
	if err := Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	cases := []struct{ role, legacy, declared string }{
		{roleChatThread, "chapter_id", "structure_id"},
		{roleChatMessage, "chapter_id", "structure_id"},
		{roleDMParticipant, "member_id", "actor_id"},
		{roleDMReaction, "member_id", "actor_id"},
		{roleDMDeviceKey, "member_id", "actor_id"},
		{roleReport, "chapter_id", "structure_id"},
		{roleRemoval, "chapter_id", "structure_id"},
		{roleDismissal, "chapter_id", "structure_id"},
		{roleDispute, "review_chapter_id", "review_structure_id"},
		{roleAddress, "member_id", "actor_id"},
		{roleAddressBlock, "member_id", "actor_id"},
		{roleNotification, "member_id", "actor_id"},
		{roleNotification, "author_member_id", "author_actor_id"},
		{roleNotification, "seat_chapter_id", "seat_structure_id"},
		{roleNotificationPreference, "member_id", "actor_id"},
	}

	for _, c := range cases {
		o, ok := s.ByRole(c.role)
		if !ok {
			t.Fatalf("role %q fills nothing", c.role)
		}
		table := s.TableFor(o)

		declared, err := hasColumn(db, table, c.declared)
		if err != nil {
			t.Fatalf("read columns of %s: %v", table, err)
		}
		if !declared {
			t.Errorf("%s has no %s column, so the store would read it back empty", table, c.declared)
		}
		legacy, err := hasColumn(db, table, c.legacy)
		if err != nil {
			t.Fatalf("read columns of %s: %v", table, err)
		}
		if legacy {
			t.Errorf("%s still carries %s beside %s, so every write would fill only one of them", table, c.legacy, c.declared)
		}
	}
}

func TestProvisionRefusesToStrandRowsInALegacyTable(t *testing.T) {
	db := provisionTestDB(t)
	s := testSpec(t)

	if err := gormstore.Migrate(db, gormstore.DefaultTableNames()); err != nil {
		t.Fatalf("seed the pre-spec table set: %v", err)
	}
	if err := spec.Migrate(db, s); err != nil {
		t.Fatalf("create the declared set beside it: %v", err)
	}
	legacy := legacyTables[roleChatThread]
	if err := db.Table(legacy).Create(map[string]any{
		"id": "cthread-1", "chapter_id": "structure-1", "tag_path": "general", "created_at": "2026-09-28T00:00:00Z",
	}).Error; err != nil {
		t.Fatalf("seed a stranded row: %v", err)
	}

	err := Provision(db, s)
	if err == nil {
		t.Fatal("Provision accepted a legacy table still holding rows that no read would reach again")
	}
	if !strings.Contains(err.Error(), legacy) || !strings.Contains(err.Error(), "still holds") {
		t.Fatalf("Provision refused for the wrong reason: %v", err)
	}
}
