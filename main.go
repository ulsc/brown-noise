package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ebitengine/oto/v3"
)

const (
	sampleRate      = 44100
	channelNum      = 2
	bitDepthInBytes = 2
	bytesPerFrame   = channelNum * bitDepthInBytes
	noiseAlpha      = 0.01

	// Adjust this duration to experiment with latency and CPU usage.
	// Smaller durations => lower latency but more playback overhead.
	// Larger durations => higher latency but less playback overhead.
	bufferDuration = 100 * time.Millisecond
)

// brownNoiseGenerator owns both its random source and filter state. Instances
// are independent, but an individual generator must not be used concurrently.
type brownNoiseGenerator struct {
	random     *rand.Rand
	alpha      float64
	lastSample float64
}

// brownNoiseReader turns the frame-oriented generator into an arbitrary byte
// stream, retaining any partial frame that does not fit in the caller's buffer.
type brownNoiseReader struct {
	generator     *brownNoiseGenerator
	pendingFrame  [bytesPerFrame]byte
	pendingOffset int
}

func main() {
	alpha := flag.Float64(
		"alpha",
		noiseAlpha,
		"brown-noise filter coefficient in the range (0, 1]; lower values sound deeper",
	)
	flag.Parse()

	if err := run(*alpha); err != nil {
		log.Fatal(err)
	}
}

func run(alpha float64) error {
	generator, err := newBrownNoiseGenerator(
		rand.New(rand.NewSource(time.Now().UnixNano())),
		alpha,
	)
	if err != nil {
		return err
	}

	context, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channelNum,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   bufferDuration,
	})
	if err != nil {
		return fmt.Errorf("create audio context: %w", err)
	}
	<-ready
	defer context.Suspend()

	player := context.NewPlayer(&brownNoiseReader{generator: generator})
	player.SetBufferSize(
		int(float64(sampleRate)*bufferDuration.Seconds()) * bytesPerFrame,
	)
	player.Play()

	ticker := time.NewTicker(bufferDuration)
	defer ticker.Stop()
	for range ticker.C {
		if err := player.Err(); err != nil {
			return fmt.Errorf("play audio: %w", err)
		}
		if err := context.Err(); err != nil {
			return fmt.Errorf("audio context: %w", err)
		}
		if !player.IsPlaying() {
			return fmt.Errorf("audio player stopped unexpectedly")
		}
	}

	return nil
}

func newBrownNoiseGenerator(r *rand.Rand, alpha float64) (*brownNoiseGenerator, error) {
	if r == nil {
		return nil, fmt.Errorf("random source must not be nil")
	}
	if !(alpha > 0 && alpha <= 1) {
		return nil, fmt.Errorf("alpha must be in the range (0, 1], got %v", alpha)
	}
	return &brownNoiseGenerator{random: r, alpha: alpha}, nil
}

// fill writes interleaved 16-bit little-endian stereo samples. Both channels
// receive the same sample so the noise remains centered in the stereo image.
func (g *brownNoiseGenerator) fill(buffer []byte) error {
	if len(buffer)%bytesPerFrame != 0 {
		return fmt.Errorf(
			"buffer length must be a multiple of %d bytes, got %d",
			bytesPerFrame,
			len(buffer),
		)
	}

	for i := 0; i < len(buffer); i += bytesPerFrame {
		randomSample := 2*g.random.Float64() - 1
		g.lastSample = g.alpha*randomSample + (1-g.alpha)*g.lastSample

		sample := int16(g.lastSample * 32767)
		stereoSample := uint32(uint16(sample))
		binary.LittleEndian.PutUint32(
			buffer[i:i+bytesPerFrame],
			(stereoSample<<16)|stereoSample,
		)
	}

	return nil
}

func (r *brownNoiseReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}

	written := 0
	if r.pendingOffset > 0 {
		copied := copy(buffer, r.pendingFrame[r.pendingOffset:])
		r.pendingOffset += copied
		written += copied
		if r.pendingOffset < bytesPerFrame {
			return written, nil
		}
		r.pendingOffset = 0
	}

	fullFrameBytes := (len(buffer) - written) / bytesPerFrame * bytesPerFrame
	if fullFrameBytes > 0 {
		if err := r.generator.fill(buffer[written : written+fullFrameBytes]); err != nil {
			return written, err
		}
		written += fullFrameBytes
	}

	if written < len(buffer) {
		if err := r.generator.fill(r.pendingFrame[:]); err != nil {
			return written, err
		}
		copied := copy(buffer[written:], r.pendingFrame[:])
		r.pendingOffset = copied
		written += copied
	}

	return written, nil
}
