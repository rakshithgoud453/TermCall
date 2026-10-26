package crypto

import "sync"

// ReplayWindow tracks seen sequence numbers over a sliding window
// to prevent UDP packet replay attacks (standard RFC 6479 / IPSec algorithm).
type ReplayWindow struct {
	mu     sync.Mutex
	maxSeq uint64
	bitmap []uint64 // 64 bits per element
	size   uint64   // Total window capacity in packets
}

// NewReplayWindow creates a window of given capacity (rounded up to multiples of 64)
func NewReplayWindow(size uint64) *ReplayWindow {
	if size < 64 {
		size = 64
	}
	words := (size + 63) / 64
	return &ReplayWindow{
		bitmap: make([]uint64, words),
		size:   words * 64,
	}
}

// CheckAndAccept checks if seq was already seen.
// If not seen, it marks it seen and returns true. If seen or too old, returns false.
func (w *ReplayWindow) CheckAndAccept(seq uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if seq == 0 {
		return false
	}

	// First packet received
	if w.maxSeq == 0 {
		w.maxSeq = seq
		w.setBit(0)
		return true
	}

	// Case 1: Newer packet advancing the window
	if seq > w.maxSeq {
		diff := seq - w.maxSeq
		if diff >= w.size {
			// Entire window passed; reset all bits
			for i := range w.bitmap {
				w.bitmap[i] = 0
			}
		} else {
			// Shift window left by diff bits
			w.shift(diff)
		}
		w.maxSeq = seq
		w.setBit(0)
		return true
	}

	// Case 2: Out of order packet within the sliding window
	diff := w.maxSeq - seq
	if diff >= w.size {
		// Packet is older than window capacity
		return false
	}

	if w.isBitSet(diff) {
		// Replay detected!
		return false
	}

	w.setBit(diff)
	return true
}

func (w *ReplayWindow) setBit(offset uint64) {
	word := offset / 64
	bit := offset % 64
	if word < uint64(len(w.bitmap)) {
		w.bitmap[word] |= 1 << bit
	}
}

func (w *ReplayWindow) isBitSet(offset uint64) bool {
	word := offset / 64
	bit := offset % 64
	if word < uint64(len(w.bitmap)) {
		return (w.bitmap[word] & (1 << bit)) != 0
	}
	return true
}

func (w *ReplayWindow) shift(diff uint64) {
	wordShift := diff / 64
	bitShift := diff % 64

	numWords := uint64(len(w.bitmap))

	for i := numWords - 1; ; i-- {
		var newWord uint64
		if i >= wordShift {
			sourceWord := i - wordShift
			newWord = w.bitmap[sourceWord] << bitShift
			if bitShift > 0 && sourceWord > 0 {
				newWord |= w.bitmap[sourceWord-1] >> (64 - bitShift)
			}
		}
		w.bitmap[i] = newWord
		if i == 0 {
			break
		}
	}
}
