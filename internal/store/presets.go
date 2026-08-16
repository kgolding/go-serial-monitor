// Package store persists the user's reusable send snippets ("presets") as
// JSON using Fyne's app storage, so it works the same way on desktop and
// mobile builds.
package store

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// Format is the encoding a Preset's Data is written in.
type Format string

const (
	FormatASCII Format = "ascii"
	FormatHex   Format = "hex"
)

// LineEnding is a line terminator that can be appended when sending data.
type LineEnding string

const (
	LineEndingNone LineEnding = "none"
	LineEndingLF   LineEnding = "lf"
	LineEndingCR   LineEnding = "cr"
	LineEndingCRLF LineEnding = "crlf"
)

// Bytes returns the raw bytes for a line ending.
func (l LineEnding) Bytes() []byte {
	switch l {
	case LineEndingLF:
		return []byte{'\n'}
	case LineEndingCR:
		return []byte{'\r'}
	case LineEndingCRLF:
		return []byte{'\r', '\n'}
	default:
		return nil
	}
}

// LineEndingOptions lists every supported LineEnding paired with a
// human-readable label, in display order.
var LineEndingOptions = []struct {
	Value LineEnding
	Label string
}{
	{LineEndingNone, "None"},
	{LineEndingLF, "LF (\\n)"},
	{LineEndingCR, "CR (\\r)"},
	{LineEndingCRLF, "CRLF (\\r\\n)"},
}

// Preset is a single stored, reusable snippet the user can send with one
// click.
type Preset struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Format     Format     `json:"format"`
	Data       string     `json:"data"`
	LineEnding LineEnding `json:"lineEnding"`
}

const presetsFileName = "presets.json"

// Store loads and saves the preset list, keeping it in memory in between.
type Store struct {
	mu      sync.Mutex
	uri     fyne.URI
	Presets []Preset
}

// NewStore creates a Store rooted at the given Fyne application's storage
// area and loads any presets already saved there.
func NewStore(a fyne.App) (*Store, error) {
	uri, err := storage.Child(a.Storage().RootURI(), presetsFileName)
	if err != nil {
		return nil, err
	}
	s := &Store{uri: uri}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Load (re)reads the preset list from disk. A missing file is not an error;
// Presets is simply left empty.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exists, err := storage.Exists(s.uri)
	if err != nil {
		return err
	}
	if !exists {
		s.Presets = nil
		return nil
	}

	r, err := storage.Reader(s.uri)
	if err != nil {
		return err
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		s.Presets = nil
		return nil
	}

	var presets []Preset
	if err := json.Unmarshal(data, &presets); err != nil {
		return err
	}
	s.Presets = presets
	return nil
}

// save writes the current preset list to disk. Callers must hold s.mu.
func (s *Store) save() error {
	w, err := storage.Writer(s.uri)
	if err != nil {
		return err
	}
	defer w.Close()

	data, err := json.MarshalIndent(s.Presets, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// All returns a copy of the current preset list.
func (s *Store) All() []Preset {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Preset, len(s.Presets))
	copy(out, s.Presets)
	return out
}

// Add appends a new preset (an ID is generated if empty) and persists it.
func (s *Store) Add(p Preset) (Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = newID()
	}
	s.Presets = append(s.Presets, p)
	if err := s.save(); err != nil {
		return p, err
	}
	return p, nil
}

// Update replaces the preset with matching ID and persists the change.
func (s *Store) Update(p Preset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Presets {
		if s.Presets[i].ID == p.ID {
			s.Presets[i] = p
			return s.save()
		}
	}
	return nil
}

// Delete removes the preset with the given ID and persists the change.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Presets {
		if s.Presets[i].ID == id {
			s.Presets = append(s.Presets[:i], s.Presets[i+1:]...)
			return s.save()
		}
	}
	return nil
}

func newID() string {
	return time.Now().Format("20060102150405.000000000")
}
