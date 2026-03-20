package compatrand

import "sync"

const (
	modulus = 2147483647
	degree  = 31
	sep     = 3
	warmup  = degree * 10
)

// RNG emulates the libc rand() stream used by Quake on Linux/glibc.
// The stream is process-global in C, so the package also exposes a shared instance.
type RNG struct {
	mu    sync.Mutex
	state [degree]uint32
	fptr  int
	rptr  int
}

func New(seed ...int32) *RNG {
	r := &RNG{}
	if len(seed) > 0 {
		r.Seed(seed[0])
	} else {
		r.Seed(1)
	}
	return r
}

func NewSeed(seed int32) *RNG {
	return New(seed)
}

func (r *RNG) Seed(seed int32) {
	if seed == 0 {
		seed = 1
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.state[0] = uint32(seed)
	for i := 1; i < degree; i++ {
		r.state[i] = uint32((uint64(16807) * uint64(r.state[i-1])) % modulus)
	}
	r.fptr = sep
	r.rptr = 0
	for i := 0; i < warmup; i++ {
		r.nextLocked()
	}
}

func (r *RNG) Int31() int32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int32(r.nextLocked())
}

func (r *RNG) Int() int32 {
	return r.Int31()
}

func (r *RNG) nextLocked() uint32 {
	r.state[r.fptr] += r.state[r.rptr]
	value := (r.state[r.fptr] >> 1) & 0x7fffffff
	r.fptr++
	if r.fptr == degree {
		r.fptr = 0
	}
	r.rptr++
	if r.rptr == degree {
		r.rptr = 0
	}
	return value
}

var shared = New(1)

func Shared() *RNG {
	return shared
}

func ResetShared(seed int32) {
	shared.Seed(seed)
}

func Int31() int32 {
	return shared.Int31()
}

func Int() int32 {
	return shared.Int()
}
