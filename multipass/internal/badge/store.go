package badge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"multipass/internal/vcb"
)

// Status is where a badge stands. Only a lasered badge opens a member's card.
type Status string

const (
	Lasered Status = "lasered" // made, and presumed on someone's lanyard
	Failed  Status = "failed"  // the laser job went wrong; never existed
	Lost    Status = "lost"    // reported lost by the member or staff
)

// Record is one issued badge.
type Record struct {
	Serial   int        `json:"serial"`
	MemberID string     `json:"member_id"`
	Name     string     `json:"name"` // as engraved
	IssuedAt time.Time  `json:"issued_at"`
	IssuedBy string     `json:"issued_by"`
	Status   Status     `json:"status"`
	VoidedAt *time.Time `json:"voided_at,omitempty"`
	VoidedBy string     `json:"voided_by,omitempty"`
	// StatusIndex is the badge's place in the published status lists,
	// random so it says nothing about who holds it or when it was made
	StatusIndex uint32 `json:"status_index"`
}

// Active reports whether the badge still opens the member's card.
func (r Record) Active() bool { return r.Status == Lasered }

var (
	ErrActiveBadge = errors.New("member already has an active badge; mark it lost or failed first")
	ErrNotActive   = errors.New("badge is not active")
	ErrNoBadge     = errors.New("no such badge")
)

// Store is the record of every badge issued, kept as one JSON file. A
// makerspace issues a few hundred badges, not millions; a file that can be
// read, backed up and fixed by hand suits that better than a database.
type Store struct {
	path    string
	mu      sync.Mutex
	data    storeFile
	version int // counts saves, for caching what's derived from the store
}

type storeFile struct {
	NextSerial int      `json:"next_serial"`
	Badges     []Record `json:"badges"`
}

// OpenStore loads the store at path, starting an empty one if there is none.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, data: storeFile{NextSerial: 1}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("badge store %s: %w", path, err)
	}
	return s, nil
}

// Issue records a new badge for a member, who may hold one active badge at a time.
func (s *Store) Issue(memberID, name, by string, now time.Time) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.data.Badges {
		if r.MemberID == memberID && r.Active() {
			return Record{}, ErrActiveBadge
		}
	}
	r := Record{
		Serial:      s.data.NextSerial,
		MemberID:    memberID,
		Name:        name,
		IssuedAt:    now.UTC(),
		IssuedBy:    by,
		Status:      Lasered,
		StatusIndex: s.newStatusIndex(),
	}
	s.data.NextSerial++
	s.data.Badges = append(s.data.Badges, r)
	if err := s.save(); err != nil {
		s.data.NextSerial--
		s.data.Badges = s.data.Badges[:len(s.data.Badges)-1]
		return Record{}, err
	}
	return r, nil
}

// Get returns the badge with the given serial.
func (s *Store) Get(serial int) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.index(serial); i >= 0 {
		return s.data.Badges[i], true
	}
	return Record{}, false
}

// ForMember returns a member's badges, newest first.
func (s *Store) ForMember(memberID string) []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Record
	for _, r := range s.data.Badges {
		if r.MemberID == memberID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Serial > out[j].Serial })
	return out
}

// Void marks an active badge failed or lost. It can't be undone: a badge found
// again is simply replaced.
func (s *Store) Void(serial int, status Status, by string, now time.Time) (Record, error) {
	if status != Failed && status != Lost {
		return Record{}, fmt.Errorf("can't void a badge as %q", status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(serial)
	if i < 0 {
		return Record{}, ErrNoBadge
	}
	old := s.data.Badges[i]
	if !old.Active() {
		return Record{}, ErrNotActive
	}
	r := old
	t := now.UTC()
	r.Status, r.VoidedAt, r.VoidedBy = status, &t, by
	s.data.Badges[i] = r
	if err := s.save(); err != nil {
		s.data.Badges[i] = old
		return Record{}, err
	}
	return r, nil
}

// Voided returns the status indexes of every badge that no longer works,
// and the store's version, which changes whenever the store does.
func (s *Store) Voided() ([]uint32, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []uint32
	for _, r := range s.data.Badges {
		if !r.Active() {
			out = append(out, r.StatusIndex)
		}
	}
	return out, s.version
}

func (s *Store) newStatusIndex() uint32 {
	for {
		idx := vcb.RandomStatusIndex()
		taken := false
		for _, r := range s.data.Badges {
			taken = taken || r.StatusIndex == idx
		}
		if !taken {
			return idx
		}
	}
}

func (s *Store) index(serial int) int {
	for i, r := range s.data.Badges {
		if r.Serial == serial {
			return i
		}
	}
	return -1
}

// save writes the whole store to a temporary file and renames it into place,
// so a crash mid-write leaves the old store, never half of a new one.
func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".badges-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return err
	}
	s.version++
	return nil
}
