package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/ebitengine/oto/v3"
)

const testSeed = 42

func TestBrownNoiseIsDeterministicForASeed(t *testing.T) {
	first := newTestGenerator(t, testSeed, noiseAlpha)
	second := newTestGenerator(t, testSeed, noiseAlpha)
	firstBuffer := make([]byte, 256*bytesPerFrame)
	secondBuffer := make([]byte, len(firstBuffer))

	if err := first.fill(firstBuffer); err != nil {
		t.Fatal(err)
	}
	if err := second.fill(secondBuffer); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(firstBuffer, secondBuffer) {
		t.Fatal("generators with the same seed produced different output")
	}
}

func TestBrownNoiseContinuityAcrossBuffers(t *testing.T) {
	separate := newTestGenerator(t, testSeed, noiseAlpha)
	combined := newTestGenerator(t, testSeed, noiseAlpha)
	first := make([]byte, 127*bytesPerFrame)
	second := make([]byte, 193*bytesPerFrame)
	whole := make([]byte, len(first)+len(second))

	if err := separate.fill(first); err != nil {
		t.Fatal(err)
	}
	if err := separate.fill(second); err != nil {
		t.Fatal(err)
	}
	if err := combined.fill(whole); err != nil {
		t.Fatal(err)
	}

	got := append(append([]byte(nil), first...), second...)
	if !bytes.Equal(got, whole) {
		t.Fatal("splitting generation across buffers changed the sample stream")
	}
}

func TestBrownNoiseWritesIdenticalStereoChannels(t *testing.T) {
	generator := newTestGenerator(t, testSeed, noiseAlpha)
	buffer := make([]byte, 512*bytesPerFrame)

	if err := generator.fill(buffer); err != nil {
		t.Fatal(err)
	}

	nonZeroSamples := 0
	for i := 0; i < len(buffer); i += bytesPerFrame {
		left := int16(binary.LittleEndian.Uint16(buffer[i : i+bitDepthInBytes]))
		right := int16(binary.LittleEndian.Uint16(
			buffer[i+bitDepthInBytes : i+bytesPerFrame],
		))
		if left != right {
			t.Fatalf("frame %d differs between channels: left=%d right=%d", i/bytesPerFrame, left, right)
		}
		if left != 0 {
			nonZeroSamples++
		}
	}
	if nonZeroSamples == 0 {
		t.Fatal("generator produced only silence")
	}
}

func TestBrownNoiseStatistics(t *testing.T) {
	for _, alpha := range []float64{0.005, 0.01, 0.05, 0.1} {
		t.Run(formatAlpha(alpha), func(t *testing.T) {
			generator := newTestGenerator(t, testSeed, alpha)
			// One second is long enough for stable, repeatable statistical checks.
			buffer := make([]byte, sampleRate*bytesPerFrame)
			if err := generator.fill(buffer); err != nil {
				t.Fatal(err)
			}

			mean, standardDeviation := sampleStatistics(buffer)
			if math.Abs(mean) > 1500 {
				t.Errorf("mean is unexpectedly far from zero: %f", mean)
			}
			if standardDeviation < 300 || standardDeviation > 6000 {
				t.Errorf("standard deviation is outside the expected range: %f", standardDeviation)
			}
		})
	}
}

func TestGeneratorsHaveIndependentState(t *testing.T) {
	const generatorCount = 4
	const samplesPerGenerator = 1024

	expected := make([][]byte, generatorCount)
	for i := range expected {
		generator := newTestGenerator(t, int64(testSeed+i), noiseAlpha)
		expected[i] = make([]byte, samplesPerGenerator*bytesPerFrame)
		if err := generator.fill(expected[i]); err != nil {
			t.Fatal(err)
		}
	}

	actual := make([][]byte, generatorCount)
	actualGenerators := make([]*brownNoiseGenerator, generatorCount)
	for i := range actualGenerators {
		actualGenerators[i] = newTestGenerator(t, int64(testSeed+i), noiseAlpha)
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(generatorCount)
	for i := range actual {
		go func(index int) {
			defer waitGroup.Done()
			actual[index] = make([]byte, samplesPerGenerator*bytesPerFrame)
			if err := actualGenerators[index].fill(actual[index]); err != nil {
				t.Errorf("generator %d: %v", index, err)
			}
		}(i)
	}
	waitGroup.Wait()

	for i := range actual {
		if !bytes.Equal(actual[i], expected[i]) {
			t.Errorf("generator %d was affected by another generator", i)
		}
	}
}

func TestNewBrownNoiseGeneratorRejectsInvalidConfiguration(t *testing.T) {
	random := rand.New(rand.NewSource(testSeed))
	for _, alpha := range []float64{-1, 0, 1.01, math.NaN()} {
		if _, err := newBrownNoiseGenerator(random, alpha); err == nil {
			t.Errorf("newBrownNoiseGenerator accepted alpha %v", alpha)
		}
	}
	if _, err := newBrownNoiseGenerator(nil, noiseAlpha); err == nil {
		t.Error("newBrownNoiseGenerator accepted a nil random source")
	}
}

func TestFillRejectsPartialFrame(t *testing.T) {
	generator := newTestGenerator(t, testSeed, noiseAlpha)
	if err := generator.fill(make([]byte, bytesPerFrame+1)); err == nil {
		t.Fatal("fill accepted a buffer containing a partial frame")
	}
}

func TestBrownNoiseReaderPreservesPartialFrames(t *testing.T) {
	const frameCount = 257
	expectedGenerator := newTestGenerator(t, testSeed, noiseAlpha)
	expected := make([]byte, frameCount*bytesPerFrame)
	if err := expectedGenerator.fill(expected); err != nil {
		t.Fatal(err)
	}

	reader := &brownNoiseReader{
		generator: newTestGenerator(t, testSeed, noiseAlpha),
	}
	chunkSizes := []int{1, 2, 3, 5, 8, 17, 31}
	actual := make([]byte, 0, len(expected))
	for readNumber := 0; len(actual) < len(expected); readNumber++ {
		chunkSize := chunkSizes[readNumber%len(chunkSizes)]
		if remaining := len(expected) - len(actual); chunkSize > remaining {
			chunkSize = remaining
		}
		chunk := make([]byte, chunkSize)
		n, err := reader.Read(chunk)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(chunk) {
			t.Fatalf("read %d bytes, want %d", n, len(chunk))
		}
		actual = append(actual, chunk...)
	}

	if !bytes.Equal(actual, expected) {
		t.Fatal("arbitrary read sizes changed the generated PCM stream")
	}
}

func TestOtoContextAndPlayer(t *testing.T) {
	if os.Getenv("BROWN_NOISE_AUDIO_TEST") != "1" {
		t.Skip("set BROWN_NOISE_AUDIO_TEST=1 to run the audio-device integration test")
	}

	context, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channelNum,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   bufferDuration,
	})
	if err != nil {
		t.Fatalf("create Oto context: %v", err)
	}
	<-ready
	defer context.Suspend()

	player := context.NewPlayer(bytes.NewReader(make([]byte, bytesPerFrame)))
	if player == nil {
		t.Fatal("create Oto player: got nil")
	}
}

func BenchmarkBrownNoiseGenerator(b *testing.B) {
	generator, err := newBrownNoiseGenerator(
		rand.New(rand.NewSource(testSeed)),
		noiseAlpha,
	)
	if err != nil {
		b.Fatal(err)
	}
	bufferSizeInBytes := int(float64(sampleRate)*bufferDuration.Seconds()) * bytesPerFrame
	buffer := make([]byte, bufferSizeInBytes)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := generator.fill(buffer); err != nil {
			b.Fatal(err)
		}
	}
}

func newTestGenerator(t testing.TB, seed int64, alpha float64) *brownNoiseGenerator {
	t.Helper()
	generator, err := newBrownNoiseGenerator(rand.New(rand.NewSource(seed)), alpha)
	if err != nil {
		t.Fatal(err)
	}
	return generator
}

func sampleStatistics(buffer []byte) (mean float64, standardDeviation float64) {
	sampleCount := len(buffer) / bytesPerFrame
	for i := 0; i < len(buffer); i += bytesPerFrame {
		mean += float64(int16(binary.LittleEndian.Uint16(buffer[i : i+bitDepthInBytes])))
	}
	mean /= float64(sampleCount)

	for i := 0; i < len(buffer); i += bytesPerFrame {
		sample := float64(int16(binary.LittleEndian.Uint16(buffer[i : i+bitDepthInBytes])))
		difference := sample - mean
		standardDeviation += difference * difference
	}
	standardDeviation = math.Sqrt(standardDeviation / float64(sampleCount))
	return mean, standardDeviation
}

func formatAlpha(alpha float64) string {
	return "alpha=" + strconv.FormatFloat(alpha, 'f', 3, 64)
}
