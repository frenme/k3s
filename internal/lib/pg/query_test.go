package pg

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func pick(db *gorm.DB) (*gorm.DB, error) { return db, nil }

func TestReadWithoutReplicaUsesPrimary(t *testing.T) {
	w := &gorm.DB{}
	if got, err := Read(&DB{Write: w, Read: w}, pick); err != nil || got != w {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestReadPrefersReplica(t *testing.T) {
	w, r := &gorm.DB{}, &gorm.DB{}
	if got, _ := Read(&DB{Write: w, Read: r}, pick); got != r {
		t.Fatal("expected replica")
	}
}

func TestReadFallsBackToPrimary(t *testing.T) {
	w, r := &gorm.DB{}, &gorm.DB{}
	calls := 0
	got, err := Read(&DB{Write: w, Read: r}, func(db *gorm.DB) (*gorm.DB, error) {
		calls++
		if db == r {
			return nil, errors.New("replica down")
		}
		return db, nil
	})
	if err != nil || got != w || calls != 2 {
		t.Fatalf("got %v err %v calls %d", got, err, calls)
	}
}

func TestReadReturnsPrimaryError(t *testing.T) {
	want := errors.New("boom")
	_, err := Read(&DB{Write: &gorm.DB{}, Read: &gorm.DB{}}, func(*gorm.DB) (int, error) { return 0, want })
	if !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
}
