package sim

import (
	"fmt"
	"hash/fnv"
)

// deriveWorldSeed returns a stable per-world seed.
// If baseSeed is non-zero, it acts as a global "salt" so different runs can vary deterministically.
func deriveWorldSeed(baseSeed int64, worldID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(worldID))
	_, _ = h.Write([]byte(fmt.Sprintf("|%d", baseSeed)))
	return int64(h.Sum64())
}

