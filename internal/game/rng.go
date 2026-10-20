package game

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

type RNG struct {
	state uint64
}

func NewRNG(seed int64) *RNG {
	return &RNG{state: uint64(seed)}
}

func RandomSeed() (int64, error) {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buffer[:])), nil
}

func (rng *RNG) Next() uint64 {
	rng.state += 0x9e3779b97f4a7c15
	value := rng.state
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func (rng *RNG) Intn(max int) int {
	if max <= 0 {
		return 0
	}
	return int(rng.Next() % uint64(max))
}

func (rng *RNG) Shuffle(count int, swap func(i, j int)) {
	for i := count - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		swap(i, j)
	}
}

func MoveSeed(seed int64, move int, purpose string) int64 {
	var hash uint64 = uint64(seed) ^ uint64(move)*0x9e3779b97f4a7c15
	for _, char := range []byte(purpose) {
		hash ^= uint64(char)
		hash *= 0x100000001b3
	}
	return int64(hash)
}

func NewGameSeed(language Language, level Level, createdAtUnix int64) (int64, error) {
	seed, err := RandomSeed()
	if err != nil {
		return 0, err
	}
	seed ^= int64(len(language))*0x45d9f3b + int64(len(level))*0x119de1f3 + createdAtUnix
	return seed, nil
}

func ValidateSeed(seed int64, purpose string) error {
	if seed == 0 {
		return fmt.Errorf("invalid seed for %s", purpose)
	}
	return nil
}
