# audiokit Design

## Architecture

```mermaid
graph TD
    subgraph "Codec Decoders"
        WAV[wav]
        MP3[mp3]
        FLAC[flac]
        Vorbis[vorbis]
        Opus[opus]
    end

    subgraph "Core Interfaces"
        AD[AudioDecoder]
        AS[AudioStream]
    end

    subgraph "Playback"
        AP[AudioPlayer]
        RB[AudioFrameRingBuffer]
        PA[PortAudio]
    end

    WAV & MP3 & FLAC & Vorbis & Opus --> AD
    AD --> AP
    AD --> AS
    AP --> RB
    RB --> PA
```

## Audio Playback Pipeline

```mermaid
sequenceDiagram
    participant App
    participant Player as AudioPlayer
    participant Producer as Producer Goroutine
    participant RB as RingBuffer
    participant CB as PortAudio Callback

    App->>Player: SetDecoder(dec)
    App->>Player: Play()
    Player->>Producer: start goroutine
    loop until EOF
        Producer->>Producer: decoder.DecodeSamples()
        Producer->>RB: Write(frame)
    end
    loop real-time thread
        CB->>RB: Read(1)
        CB->>CB: copy to output buffer
    end
    App->>Player: Wait()
```

## Thread Safety Model

The `AudioPlayer` uses a **Single-Producer Single-Consumer (SPSC)** pattern:

- **Producer goroutine** - decodes audio and writes frames to the ring buffer
- **PortAudio C callback thread** - reads frames from the ring buffer and copies to output
- **Atomic operations** for all shared state (`producerDone`, `playbackComplete`, `draining`)
- **Deep copy** of frame data to prevent buffer corruption across thread boundary
