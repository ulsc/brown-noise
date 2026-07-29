# Brown Noise Generator

High-performance, low-footprint brown noise generator written in Go.

Brown noise, also known as Brownian noise or red noise, is a type of noise signal that has a power spectral density inversely proportional to the square of the frequency. This creates a noise signal with a deeper sound compared to white or pink noise, making it ideal for various applications such as sleep, relaxation, and concentration.

## Features

- Generates brown noise in real-time
- Utilizes the [Oto](https://github.com/ebitengine/oto) library for cross-platform audio playback
- Efficient noise generation algorithm
- Can be toggled on and off using a simple command or script

## Installation

Requirements:

- Go 1.25 or newer
- A working audio output device
- On Debian/Ubuntu Linux, the Oto dependency also requires `libasound2-dev`

1. Clone the repository:

```bash
git clone https://github.com/ulsc/brown-noise.git
```

2. Change to the cloned directory:

```bash
cd brown-noise
```

3. Build the Go application:

```bash
go build -o brown_noise .
```

On Windows, use:

```powershell
go build -o brown_noise.exe .
```

This generates `brown_noise` (`brown_noise.exe` on Windows). Unix and macOS users can alternatively run `make build`, or `make install` to install the binary under `/usr/local/bin`.

## Usage

### Basic usage

To start the brown noise generator on Unix or macOS:

```bash
./brown_noise
```

On Windows:

```powershell
.\brown_noise.exe
```

Press `Ctrl+C` to stop the generator.

Use `-alpha` to tune the noise depth. The default is `0.01`; lower values sound deeper, and accepted values are greater than `0` and no greater than `1`:

```bash
./brown_noise -alpha 0.005
```

On Windows:

```powershell
.\brown_noise.exe -alpha 0.005
```

### Background execution

You can run the brown noise generator in the background by using the `nohup` command:

```bash
nohup ./brown_noise > /dev/null 2>&1 &
echo $!
```

To stop instances started under the installed executable name:

```bash
pkill -x brown_noise
```

### Toggle script

The repository includes `toggle_noise.sh` for Unix and macOS. Install `brown_noise` somewhere on `PATH`, then run:

```bash
./toggle_noise.sh
```

The script uses exact process-name matching so it does not terminate unrelated commands whose arguments happen to contain `brown_noise`.

## Implementation Details

The brown noise generator uses the following key components:

- A custom `rand.Rand` instance seeded with the current Unix time in nanoseconds to ensure unique random sequences for each run.
- The Oto library for cross-platform audio playback.
- A low-pass filter implemented using an exponential moving average algorithm.

The generator creates a buffer containing audio samples with a specified sample rate, number of channels, and bit depth. It generates random samples in the range of -1 to 1, applies a low-pass filter using the exponential moving average algorithm, and writes the filtered samples to the buffer for playback.

The low-pass filter has a tunable parameter `alpha`, which determines the depth of the noise. Lower values of `alpha` result in deeper noise. By adjusting this parameter, you can fine-tune the generated noise to suit your preferences or specific use cases.

The generator continuously creates and plays back buffers of brown noise, ensuring seamless audio playback.

## Performance

The brown noise generator has been designed to minimize CPU and memory usage while providing high-quality noise generation. By generating noise in real-time and using efficient algorithms, the application can run on a wide variety of systems without causing performance issues.

Compared to playing a pre-recorded brown noise MP3 or WAV file, the real-time generation approach used in this application offers the following benefits:

- Infinite, non-repeating noise generation
- No need for large audio files or continuous looping
- Customizable noise depth through the `alpha` parameter

### Benchmarking

Run the generator benchmark on your system with:

```bash
go test -bench=BrownNoise -benchmem
```

Run the deterministic unit tests and race detector with:

```bash
go test ./...
go test -race ./...
```

The audio-device integration test is opt-in because it opens the system audio device:

```bash
BROWN_NOISE_AUDIO_TEST=1 go test -run TestOtoContextAndPlayer
```

## Contributing

Contributions are welcome! If you have suggestions for improvements, bug reports, or new features, please create an issue or submit a pull request on GitHub.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
