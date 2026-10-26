package jitter

import (
	"math"
	"sync"
)

// Frame holds an incoming audio packet and its sequence number
type Frame struct {
	Seq       uint64
	Timestamp uint16
	Data      []byte
}

// Stats provides real-time jitter and packet metrics for telemetry
type Stats struct {
	PacketsReceived  uint64
	PacketsLost      uint64
	PacketsConcealed uint64
	CurrentJitterMs  float64
	BufferDepthMs    int
}

// JitterBuffer reorders incoming UDP audio packets and provides a steady stream
// of frames at 20ms intervals, triggering Packet Loss Concealment (PLC) on gaps.
type JitterBuffer struct {
	mu           sync.Mutex
	frames       map[uint64]*Frame
	nextPlaySeq  uint64
	highestSeq   uint64
	initialized  bool
	playing      bool
	targetDepth  int // Number of frames to buffer before starting playback (e.g. 2 frames = 40ms)
	maxCapacity  int // Max frames stored to prevent runaway memory (e.g. 50 frames = 1s)

	// Jitter calculation (RFC 3550)
	lastTransit  int64
	jitterEstimate float64

	// Metrics
	stats Stats
}

// NewJitterBuffer creates an adaptive jitter buffer
func NewJitterBuffer(targetDepthFrames, maxCapacityFrames int) *JitterBuffer {
	if targetDepthFrames <= 0 {
		targetDepthFrames = 2 // 40ms target delay
	}
	if maxCapacityFrames <= targetDepthFrames {
		maxCapacityFrames = 50 // 1 second max
	}
	return &JitterBuffer{
		frames:      make(map[uint64]*Frame),
		targetDepth: targetDepthFrames,
		maxCapacity: maxCapacityFrames,
	}
}

// Push inserts an incoming audio frame into the jitter buffer
func (jb *JitterBuffer) Push(seq uint64, timestamp uint16, data []byte, localTransitMs int64) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	jb.stats.PacketsReceived++

	// RFC 3550 Jitter Estimate:
	// D(i, j) = (R_j - S_j) - (R_i - S_i)
	// J = J + (|D| - J) / 16
	if jb.lastTransit != 0 {
		diff := math.Abs(float64(localTransitMs - jb.lastTransit))
		jb.jitterEstimate += (diff - jb.jitterEstimate) / 16.0
		jb.stats.CurrentJitterMs = jb.jitterEstimate
	}
	jb.lastTransit = localTransitMs

	if !jb.initialized {
		jb.nextPlaySeq = seq
		jb.highestSeq = seq
		jb.initialized = true
	}

	if seq > jb.highestSeq {
		jb.highestSeq = seq
	}

	// Discard frames that are older than what we have already played
	if seq < jb.nextPlaySeq {
		return
	}

	// Store frame
	frameCopy := make([]byte, len(data))
	copy(frameCopy, data)
	jb.frames[seq] = &Frame{
		Seq:       seq,
		Timestamp: timestamp,
		Data:      frameCopy,
	}

	// Evict oldest if exceeding maxCapacity
	if len(jb.frames) > jb.maxCapacity {
		var oldestSeq uint64 = math.MaxUint64
		for s := range jb.frames {
			if s < oldestSeq {
				oldestSeq = s
			}
		}
		delete(jb.frames, oldestSeq)
	}
}

// Pop retrieves the next frame in sequence for the audio output device.
// If the buffer hasn't reached targetDepth yet, returns nil, false (buffering).
// If the next expected frame is missing, advances sequence, increments loss stats,
// and returns nil, true (signal to invoke Opus PLC).
func (jb *JitterBuffer) Pop() (data []byte, isPLC bool) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	if !jb.initialized {
		return nil, false
	}

	// Wait until buffer reaches initial target depth before starting initial drain
	if !jb.playing {
		if len(jb.frames) < jb.targetDepth {
			return nil, false
		}
		jb.playing = true
	}

	frame, exists := jb.frames[jb.nextPlaySeq]
	if exists {
		delete(jb.frames, jb.nextPlaySeq)
		jb.nextPlaySeq++
		jb.stats.BufferDepthMs = len(jb.frames) * 20
		return frame.Data, false
	}

	// Frame missing: if we haven't reached the highest received sequence,
	// this is a dropped packet. Increment lost and signal PLC.
	if jb.nextPlaySeq < jb.highestSeq {
		jb.stats.PacketsLost++
		jb.stats.PacketsConcealed++
		jb.nextPlaySeq++
		jb.stats.BufferDepthMs = len(jb.frames) * 20
		return nil, true
	}

	// Starvation / underrun
	jb.stats.BufferDepthMs = len(jb.frames) * 20
	return nil, false
}

// Stats returns a snapshot of current telemetry
func (jb *JitterBuffer) Stats() Stats {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	return jb.stats
}

// SetTargetDepth dynamically adjusts the target buffer depth (in frames)
func (jb *JitterBuffer) SetTargetDepth(depth int) {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	if depth < 1 {
		depth = 1
	} else if depth > jb.maxCapacity/2 {
		depth = jb.maxCapacity / 2
	}
	jb.targetDepth = depth
}

// TargetDepth returns the current target depth in frames
func (jb *JitterBuffer) TargetDepth() int {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	return jb.targetDepth
}

// Reset clears the buffer
func (jb *JitterBuffer) Reset() {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	jb.frames = make(map[uint64]*Frame)
	jb.nextPlaySeq = 0
	jb.highestSeq = 0
	jb.initialized = false
	jb.playing = false
	jb.lastTransit = 0
	jb.jitterEstimate = 0
	jb.stats = Stats{}
}
