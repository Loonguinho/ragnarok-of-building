// Package view computes what each window of the simulator shows (the build,
// equipment, buffs, damage, skill tree, optimizer, replay recap...) as plain
// structs, with the interface texts translated. internal/ui draws them.
package view

import (
	"io"
	"log/slog"
	"sync"

	"ragnarok-of-building/internal/divinepride"
	"ragnarok-of-building/internal/gamedata"
)

// Options configure the views.
type Options struct {
	Log     *slog.Logger      // nil: discard
	Catalog *gamedata.Catalog // the game data
	// DP fetches, one ID at a time, monsters rAthena has no stats for
	// (nil: no Divine Pride key).
	DP *divinepride.Client
	// PasteURL is the paste service for shared builds (empty: paste.rs).
	PasteURL string
}

// Server holds the catalog and what the views keep between calls.
type Server struct {
	opt   Options
	items *gamedata.ItemIndex

	// Optimizer card pools, built on first use.
	optOnce     sync.Once
	optCardList []*gamedata.Item

	// Monster stats being fetched from Divine Pride, and IDs that failed.
	mobMu     sync.Mutex
	mobFailed map[int]bool
}

// New prepares the views over a catalog.
func New(opt Options) (*Views, error) {
	if opt.Log == nil {
		opt.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s := &Server{opt: opt}
	if opt.Catalog != nil {
		s.items = gamedata.NewItemIndex(opt.Catalog)
	}
	return &Views{s: s}, nil
}
