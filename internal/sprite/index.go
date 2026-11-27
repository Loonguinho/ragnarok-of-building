package sprite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"strconv"
	"sync"
)

// The web page ships the pictures cmd/spriteimport drew from the client
// (one PNG per distinct picture, named by its contents) and an index of
// them, and puts the character together from those.

// IndexFile is the index's name in the sprites folder.
const IndexFile = "index.json"

// Entry is one drawn frame: its file and where its origin and anchor are.
type Entry struct {
	File      string `json:"f"`
	OX        int    `json:"ox"`
	OY        int    `json:"oy"`
	AX        int    `json:"ax,omitempty"`
	AY        int    `json:"ay,omitempty"`
	HasAnchor bool   `json:"a,omitempty"`
}

// Index lists the drawn frames: bodies by "<job ID>_m|f", heads by
// "<style>_m|f" (Doram heads "d<style>_m|f") and headgears by
// "<view ID>_m|f" (an item's ClassNum).
type Index struct {
	Body map[string]Entry `json:"body"`
	Head map[string]Entry `json:"head"`
	Acc  map[string]Entry `json:"acc"`
	// Doram lists the Doram jobs: their heads are their own.
	Doram []int `json:"doram,omitempty"`
}

// Key is an index key for an ID and a sex.
func Key(id int, female bool) string {
	if female {
		return strconv.Itoa(id) + "_f"
	}
	return strconv.Itoa(id) + "_m"
}

// Look is what the character wears and looks like.
type Look struct {
	Job    int
	Female bool
	Hair   int    // style, from 1
	Heads  [3]int // view IDs of the top, middle and lower headgears (0: none)
}

// Served puts characters together from a sprites folder read through get
// (the index first, once; then each picture, kept once decoded).
type Served struct {
	get func(ctx context.Context, name string) ([]byte, error)

	once  sync.Once
	index Index
	err   error

	mu   sync.Mutex
	pics map[string]Picture
}

// NewServed reads the sprites folder through get.
func NewServed(get func(ctx context.Context, name string) ([]byte, error)) *Served {
	return &Served{get: get, pics: map[string]Picture{}}
}

// Hairs is how many hair styles there are for a job and sex.
func (s *Served) Hairs(ctx context.Context, job int, female bool) int {
	if s.load(ctx) != nil {
		return 0
	}
	prefix := ""
	if s.doram(job) {
		prefix = "d"
	}
	n := 0
	for i := 1; ; i++ {
		if _, ok := s.index.Head[prefix+Key(i, female)]; !ok {
			return n
		}
		n = i
	}
}

func (s *Served) doram(job int) bool {
	for _, j := range s.index.Doram {
		if j == job {
			return true
		}
	}
	return false
}

func (s *Served) load(ctx context.Context) error {
	s.once.Do(func() {
		b, err := s.get(ctx, IndexFile)
		if err != nil {
			s.err = fmt.Errorf("sprites: %w", err)
			return
		}
		if err := json.Unmarshal(b, &s.index); err != nil {
			s.err = fmt.Errorf("sprites: %s: %w", IndexFile, err)
		}
	})
	return s.err
}

func (s *Served) picture(ctx context.Context, e Entry) (Picture, error) {
	s.mu.Lock()
	p, ok := s.pics[e.File]
	s.mu.Unlock()
	if !ok {
		b, err := s.get(ctx, e.File+".png")
		if err != nil {
			return Picture{}, err
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			return Picture{}, err
		}
		n := image.NewNRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
		draw.Draw(n, n.Rect, img, img.Bounds().Min, draw.Src)
		p.Img = n
		s.mu.Lock()
		s.pics[e.File] = p
		s.mu.Unlock()
	}
	p.Origin = image.Pt(e.OX, e.OY)
	p.Anchor, p.HasAnchor = image.Pt(e.AX, e.AY), e.HasAnchor
	return p, nil
}

// Character draws the character standing, facing the viewer: body, head,
// then the headgears (lower, top, middle, as the client draws them; one
// worn in several positions is drawn once). Pictures the client lacks
// are left out; with no body there is no character (os.ErrNotExist).
func (s *Served) Character(ctx context.Context, l Look) (*image.NRGBA, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	pick := func(m map[string]Entry, key, other string) (Entry, bool) {
		if e, ok := m[key]; ok {
			return e, true
		}
		e, ok := m[other]
		return e, ok
	}
	female := l.Female
	body, ok := s.index.Body[Key(l.Job, female)]
	if !ok { // a job with one sex only (Odalisca, Bardo...)
		female = !female
		if body, ok = s.index.Body[Key(l.Job, female)]; !ok {
			return nil, os.ErrNotExist
		}
	}
	var parts []Picture
	add := func(e Entry) error {
		p, err := s.picture(ctx, e)
		if err == nil {
			parts = append(parts, p)
		}
		return err
	}
	if err := add(body); err != nil {
		return nil, err
	}
	prefix := ""
	if s.doram(l.Job) {
		prefix = "d"
	}
	if e, ok := pick(s.index.Head, prefix+Key(max(l.Hair, 1), female), prefix+Key(1, female)); ok {
		add(e)
	}
	drawn := map[int]bool{}
	for _, i := range []int{2, 0, 1} { // lower, top, middle
		v := l.Heads[i]
		if v <= 0 || drawn[v] {
			continue
		}
		drawn[v] = true
		if e, ok := pick(s.index.Acc, Key(v, female), Key(v, !female)); ok {
			add(e)
		}
	}
	return Compose(parts...), nil
}
