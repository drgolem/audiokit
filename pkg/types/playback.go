package types

import "time"

// PlaybackStatus holds playback information for audio players.
type PlaybackStatus struct {
	FileName        string
	SampleRate      int
	Channels        int
	BitsPerSample   int
	FramesPerBuffer int
	PlayedSamples   uint64
	BufferedSamples uint64
	ElapsedTime     time.Duration
}

// PlaybackMonitor is implemented by types that can report playback status.
type PlaybackMonitor interface {
	GetPlaybackStatus() PlaybackStatus
}
