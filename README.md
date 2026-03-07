# audiokit

Go library for decoding and playing audio files. Supports multiple codecs with a unified interface.

## Supported Formats

| Format | Decoder | CGO |
|--------|---------|-----|
| WAV    | [go-wav](https://github.com/youpy/go-wav) | No |
| MP3    | [go-mp3](https://github.com/imcarsen/go-mp3) | No |
| FLAC   | [go-flac](https://github.com/drgolem/go-flac) | Yes (libflac) |
| Vorbis | [oggvorbis](https://github.com/jfreymuth/oggvorbis) | No |
| Opus   | [go-opus](https://github.com/drgolem/go-opus) | Yes (libopus) |

## Packages

- **decoder** - Unified `AudioDecoder` interface with codec-specific implementations
- **audioplayer** - PortAudio-based playback with ringbuffer (SPSC pattern, callback mode)
- **audio** - `AudioStream` interface for streaming pipelines
- **audioframe** - PCM audio frame type with format metadata
- **audioframeringbuffer** - Lock-free ring buffer for audio frames
- **types** - Shared types (format, song info, playlist, playback status)
- **oggutil** - OGG container utilities

## Usage

```go
import (
    "github.com/drgolem/audiokit/pkg/decoder"
    "github.com/drgolem/audiokit/pkg/audioplayer"
)

// Decode any supported format
dec, err := decoder.NewDecoder("song.flac")
defer dec.Close()

rate, channels, bps := dec.GetFormat()

// Play audio
player := audioplayer.New(deviceIdx, 64, 1024, 4096)
player.SetDecoder(dec, "song.flac")
player.Play()
player.Wait()
player.Stop()
```

## Example Player

A complete example audio player is included:

```sh
go run ./internal/examples/player song.flac
go run ./internal/examples/player -d 1 -v music.mp3
go run ./internal/examples/player -h   # show all flags
```

Flags: `-d` device index, `-c` buffer capacity, `-p` PA frames, `-s` samples per frame, `-v` verbose.

## Build Requirements

CGO-dependent codecs require native libraries:

```sh
# macOS
brew install flac libogg opus portaudio

# Ubuntu/Debian
apt-get install libflac-dev libogg-dev libopus-dev libopusfile-dev portaudio19-dev

# Raspberry Pi (Raspbian/Raspberry Pi OS)
sudo apt-get install libflac-dev libogg-dev libopus-dev libopusfile-dev portaudio19-dev libasound2-dev
```

## License

MIT
