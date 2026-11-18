package web

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"sync"
)

var assetVersions sync.Map // path under static/ -> short content hash

// assetURL is /static/<path>?v=<hash of the file>: a new version gets a new
// URL, so browsers never keep running an old script or stylesheet from
// their cache.
func (s *Server) assetURL(path string) string {
	if !s.opt.Dev {
		if v, ok := assetVersions.Load(path); ok {
			return "/static/" + path + "?v=" + v.(string)
		}
	}
	b, err := fs.ReadFile(s.fsys, "static/"+path)
	if err != nil {
		return "/static/" + path
	}
	sum := sha256.Sum256(b)
	v := hex.EncodeToString(sum[:4])
	if !s.opt.Dev {
		assetVersions.Store(path, v)
	}
	return "/static/" + path + "?v=" + v
}
