// Copyright 2016 The Gorilla WebSocket Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

// !appengine

package websocket

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func maskBytesByByte(key [4]byte, pos int, b []byte) int {
	for i := range b {
		b[i] ^= key[pos&3]
		pos++
	}
	return pos & 3
}

func notzero(b []byte) int {
	for i := range b {
		if b[i] != 0 {
			return i
		}
	}
	return -1
}

func TestMaskBytes(t *testing.T) {
	key := [4]byte{1, 2, 3, 4}
	for size := 1; size <= 1024; size++ {
		for align := 0; align < wordSize; align++ {
			for pos := 0; pos < 4; pos++ {
				b := make([]byte, size+align)[align:]
				maskBytes(key, pos, b)
				maskBytesByByte(key, pos, b)
				if i := notzero(b); i >= 0 {
					t.Errorf("size:%d, align:%d, pos:%d, offset:%d", size, align, pos, i)
				}
			}
		}
	}
}

func BenchmarkMaskBytes(b *testing.B) {
	for _, size := range []int{2, 4, 8, 16, 32, 512, 1024} {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			for _, align := range []int{wordSize / 2} {
				b.Run(fmt.Sprintf("align-%d", align), func(b *testing.B) {
					for _, fn := range []struct {
						name string
						fn   func(key [4]byte, pos int, b []byte) int
					}{
						{"byte", maskBytesByByte},
						{"word", maskBytes},
					} {
						b.Run(fn.name, func(b *testing.B) {
							key := newMaskKey()
							data := make([]byte, size+align)[align:]
							for i := 0; i < b.N; i++ {
								fn.fn(key, 0, data)
							}
							b.SetBytes(int64(len(data)))
						})
					}
				})
			}
		})
	}
}

type failReader struct{}

func (failReader) Read(p []byte) (int, error) {
	return 0, errors.New("read failure")
}

func TestNewMaskKeyRandomness(t *testing.T) {
	const count = 1000
	keys := make(map[[4]byte]struct{}, count)
	byteCounts := [4]map[byte]int{
		make(map[byte]int),
		make(map[byte]int),
		make(map[byte]int),
		make(map[byte]int),
	}

	for i := 0; i < count; i++ {
		k := newMaskKey()
		if k == [4]byte{0, 0, 0, 0} {
			t.Fatalf("newMaskKey returned all zeros at iteration %d", i)
		}
		keys[k] = struct{}{}
		for pos := 0; pos < 4; pos++ {
			byteCounts[pos][k[pos]]++
		}
	}

	// 1000 32-bit keys should almost certainly have zero collisions
	if len(keys) < count-5 {
		t.Errorf("low diversity in mask keys: got %d unique keys out of %d", len(keys), count)
	}

	// Each byte position must exhibit wide variance across the range
	for pos := 0; pos < 4; pos++ {
		if len(byteCounts[pos]) < 100 {
			t.Errorf("byte position %d lacks entropy: only %d distinct values across %d keys", pos, len(byteCounts[pos]), count)
		}
	}
}

func TestNewMaskKeyFallback(t *testing.T) {
	origMaskRand := maskRand
	defer func() { maskRand = origMaskRand }()

	// Test when maskRand is nil
	maskRand = nil
	k1 := newMaskKey()
	if k1 == [4]byte{0, 0, 0, 0} {
		t.Fatal("newMaskKey returned all zeros when maskRand is nil")
	}

	// Test when maskRand returns an error, it falls back to rand.Reader
	maskRand = failReader{}
	k2 := newMaskKey()
	if k2 == [4]byte{0, 0, 0, 0} {
		t.Fatal("newMaskKey returned all zeros when maskRand fails")
	}
}

func TestMaskBytesRoundtrip(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 7, 8, 15, 16, 31, 32, 63, 64, 125, 126, 127, 128, 255, 256, 1024, 4096, 65535, 65536}

	for _, size := range sizes {
		for align := 0; align < wordSize; align++ {
			original := make([]byte, size+align)[align:]
			for i := range original {
				original[i] = byte(i*31 + 7)
			}

			key := newMaskKey()

			// Roundtrip using word-aligned maskBytes
			masked := make([]byte, len(original))
			copy(masked, original)
			maskBytes(key, 0, masked)

			if len(original) > 4 && bytes.Equal(masked, original) {
				t.Fatalf("size %d align %d: masked data identical to original data", size, align)
			}

			maskBytes(key, 0, masked)
			if !bytes.Equal(masked, original) {
				t.Fatalf("size %d align %d: roundtrip failed, got != original", size, align)
			}

			// Roundtrip cross-check against maskBytesByByte reference implementation
			refMasked := make([]byte, len(original))
			copy(refMasked, original)
			maskBytesByByte(key, 0, refMasked)

			copy(masked, original)
			maskBytes(key, 0, masked)
			if !bytes.Equal(masked, refMasked) {
				t.Fatalf("size %d align %d: maskBytes output does not match byte-by-byte reference", size, align)
			}
		}
	}
}

func TestMaskKeyStreamingRoundtrip(t *testing.T) {
	const totalSize = 2048
	original := make([]byte, totalSize)
	for i := range original {
		original[i] = byte(i*17 + 13)
	}

	key := newMaskKey()
	chunkSizes := []int{1, 2, 3, 5, 7, 11, 16, 32, 64, 128}

	masked := make([]byte, totalSize)
	copy(masked, original)

	// Stream mask in chunks with position progression
	pos := 0
	offset := 0
	chunkIdx := 0
	for offset < totalSize {
		chunkLen := chunkSizes[chunkIdx%len(chunkSizes)]
		if offset+chunkLen > totalSize {
			chunkLen = totalSize - offset
		}
		pos = maskBytes(key, pos, masked[offset:offset+chunkLen])
		offset += chunkLen
		chunkIdx++
	}

	if bytes.Equal(masked, original) {
		t.Fatal("streaming masked data identical to original")
	}

	// Stream unmask in different chunk sizes
	unmaskChunkSizes := []int{4, 8, 15, 23, 50, 100}
	pos = 0
	offset = 0
	chunkIdx = 0
	for offset < totalSize {
		chunkLen := unmaskChunkSizes[chunkIdx%len(unmaskChunkSizes)]
		if offset+chunkLen > totalSize {
			chunkLen = totalSize - offset
		}
		pos = maskBytes(key, pos, masked[offset:offset+chunkLen])
		offset += chunkLen
		chunkIdx++
	}

	if !bytes.Equal(masked, original) {
		t.Fatal("streaming roundtrip failed to recover original payload")
	}
}

func TestClientFrameMaskRoundtrip(t *testing.T) {
	var connBuf bytes.Buffer
	client := newTestConn(nil, &connBuf, false) // Client masks frames
	server := newTestConn(&connBuf, nil, true)  // Server unmasks frames

	testMessages := [][]byte{
		[]byte("hello websocket world"),
		make([]byte, 125),
		make([]byte, 126),
		make([]byte, 1024),
		make([]byte, 8192),
	}
	for i := range testMessages {
		for j := range testMessages[i] {
			testMessages[i][j] = byte(i*37 + j)
		}
	}

	seenMaskKeys := make(map[[4]byte]struct{})

	for i, msg := range testMessages {
		connBuf.Reset()

		err := client.WriteMessage(BinaryMessage, msg)
		if err != nil {
			t.Fatalf("message %d: client.WriteMessage failed: %v", i, err)
		}

		wireBytes := connBuf.Bytes()
		if len(wireBytes) < 6 {
			t.Fatalf("message %d: wire bytes length %d too short for masked frame", i, len(wireBytes))
		}

		// RFC 6455 Section 5.2: Client-to-server frame MUST set mask bit (0x80) on second byte
		if wireBytes[1]&maskBit == 0 {
			t.Fatalf("message %d: client frame did not set mask bit", i)
		}

		// Locate mask key based on payload length encoding
		var keyOffset int
		payloadLenByte := wireBytes[1] & 0x7f
		switch {
		case payloadLenByte <= 125:
			keyOffset = 2
		case payloadLenByte == 126:
			keyOffset = 4
		case payloadLenByte == 127:
			keyOffset = 10
		}

		var frameKey [4]byte
		copy(frameKey[:], wireBytes[keyOffset:keyOffset+4])

		if frameKey == [4]byte{0, 0, 0, 0} {
			t.Fatalf("message %d: frame mask key is all zeros", i)
		}
		if _, exists := seenMaskKeys[frameKey]; exists {
			t.Fatalf("message %d: duplicate mask key detected across frames", i)
		}
		seenMaskKeys[frameKey] = struct{}{}

		// Server unmasks and reads the message
		msgType, readMsg, err := server.ReadMessage()
		if err != nil {
			t.Fatalf("message %d: server.ReadMessage failed: %v", i, err)
		}
		if msgType != BinaryMessage {
			t.Fatalf("message %d: got message type %d, want %d", i, msgType, BinaryMessage)
		}
		if !bytes.Equal(readMsg, msg) {
			t.Fatalf("message %d: unmasked message does not match original", i)
		}
	}
}
