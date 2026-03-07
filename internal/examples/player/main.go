// Example: single-file audio player using audiokit.
//
// Usage:
//
//	go run ./internal/examples/player song.flac
//	go run ./internal/examples/player -d 0 music.mp3
//	go run ./internal/examples/player -v song.wav
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/drgolem/audiokit/pkg/audioplayer"
	"github.com/drgolem/audiokit/pkg/decoder"
	"github.com/drgolem/audiokit/pkg/decoder/flac"
	"github.com/drgolem/audiokit/pkg/decoder/mp3"
	"github.com/drgolem/audiokit/pkg/decoder/opus"
	"github.com/drgolem/audiokit/pkg/decoder/vorbis"
	"github.com/drgolem/audiokit/pkg/decoder/wav"

	"github.com/drgolem/go-portaudio/portaudio"
)

func main() {
	deviceIdx := flag.Int("d", 1, "audio output device index")
	bufCapacity := flag.Uint64("c", 256, "ringbuffer capacity (frames)")
	paFrames := flag.Int("p", 512, "PortAudio frames per buffer")
	samplesPerFrame := flag.Int("s", 4096, "samples per AudioFrame")
	verbose := flag.Bool("v", false, "verbose output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: player [flags] <audio_file>\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSupported formats: .mp3 .flac .wav .ogg .opus\n")
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	fileName := flag.Arg(0)
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		slog.Error("File not found", "path", fileName)
		os.Exit(1)
	}

	// Initialize PortAudio
	if err := portaudio.Initialize(); err != nil {
		slog.Error("Failed to initialize PortAudio", "error", err)
		os.Exit(1)
	}
	defer portaudio.Terminate()
	slog.Info("PortAudio initialized", "version", portaudio.GetVersion())

	// Create decoder from file extension
	dec, err := newDecoder(fileName)
	if err != nil {
		slog.Error("Failed to open file", "error", err)
		os.Exit(1)
	}

	rate, channels, bps := dec.GetFormat()
	slog.Info("Audio format", "sample_rate", rate, "channels", channels, "bits_per_sample", bps)

	// Create player and start playback
	player := audioplayer.New(*deviceIdx, *bufCapacity, *paFrames, *samplesPerFrame)
	player.SetDecoder(dec, filepath.Base(fileName))

	if err := player.Play(); err != nil {
		slog.Error("Failed to start playback", "error", err)
		os.Exit(1)
	}

	// Monitor playback status in background
	statusDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				st := player.GetPlaybackStatus()
				playedSec := float64(st.PlayedSamples) / float64(st.SampleRate)
				slog.Info("Playing",
					"file", st.FileName,
					"time", fmt.Sprintf("%02d:%02d", int(playedSec)/60, int(playedSec)%60),
					"buffered", fmt.Sprintf("%.1fs", float64(st.BufferedSamples)/float64(st.SampleRate)),
				)
			case <-statusDone:
				return
			}
		}
	}()

	// Wait for playback to finish or signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		player.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Playback completed")
	case sig := <-sigChan:
		slog.Info("Interrupted", "signal", sig)
	}

	close(statusDone)
	player.Stop()
}

// newDecoder creates and opens the appropriate decoder based on file extension.
func newDecoder(fileName string) (decoder.AudioDecoder, error) {
	reg := decoder.NewRegistry()
	reg.Register(".mp3", func(int) (decoder.AudioDecoder, error) { return mp3.NewDecoder(), nil })
	reg.Register(".flac", func(bps int) (decoder.AudioDecoder, error) { return flac.NewDecoder(bps) })
	reg.Register(".fla", func(bps int) (decoder.AudioDecoder, error) { return flac.NewDecoder(bps) })
	reg.Register(".wav", func(int) (decoder.AudioDecoder, error) { return wav.NewDecoder(), nil })
	reg.Register(".ogg", func(bps int) (decoder.AudioDecoder, error) { return vorbis.NewDecoder(bps) })
	reg.Register(".oga", func(bps int) (decoder.AudioDecoder, error) { return vorbis.NewDecoder(bps) })
	reg.Register(".opus", func(int) (decoder.AudioDecoder, error) { return opus.NewDecoder(), nil })
	return reg.NewFromFile(fileName, 0)
}
