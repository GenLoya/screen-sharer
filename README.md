# screen-sharer

Stream a screen over the LAN as H.264/H.265 video over TCP, encoded and decoded with ffmpeg.

- `cmd/receiver`: listens for senders and shows the stream in a resizable window.
  Only one sender is shown at a time. When a new sender connects, the previous one is disconnected.
- `cmd/sender`: connects to a receiver and streams the display it was told to share.

## Requirements

[ffmpeg](https://ffmpeg.org) must be installed and on your `PATH` on **both** machines.
The sender uses it to encode, and the receiver uses it to decode. Both programs refuse to start without it.

```sh
# Windows
winget install Gyan.FFmpeg
# macOS
brew install ffmpeg
```

Use `-ffmpeg <path>` if ffmpeg is installed somewhere outside your `PATH`.

## Usage

On the viewing machine:

```sh
go run ./cmd/receiver -addr :9000
```

On the machine sharing its screen:

```sh
go run ./cmd/sender -addr <receiver-ip>:9000 -display 1 -codec h264 -fps 30 -bitrate 4M
```

- `-display` is 1-based (`1` is the first monitor). Run `sender -list-displays` to see the numbering.
- `-codec` is `h264` (default) or `h265`. The receiver picks up the codec automatically.
- `-encoder` overrides the ffmpeg encoder. The default is the software encoder (`libx264` / `libx265`).
  Use a hardware encoder to lower CPU usage, e.g. `h264_qsv`, `h264_nvenc`, `h264_amf` or `h264_videotoolbox`.

### Capture

ffmpeg captures the display itself. Pick the method with `-capture`:

| Platform | Default | How it works |
| --- | --- | --- |
| Windows | `ddagrab` | Desktop Duplication API (GPU). Displays are numbered in DXGI order on the primary GPU. |
| macOS | `avfoundation` | AVFoundation screen capture. Needs Screen Recording permission. |
| Any | `pipe` | Fallback: Go captures frames and pipes them into ffmpeg. Slower, but works everywhere. |

On Windows, `ddagrab` with an Intel Quick Sync encoder (`-encoder h264_qsv` or `hevc_qsv`) keeps frames on the GPU
from capture to encode. In testing it used about 13x less CPU than software encoding (0.6 vs 8 CPU-seconds per 5 seconds at 1080p30).
Every other encoder gets the frames copied to system memory first.
- The mouse pointer is sent separately from the video (about 60 updates per second) and drawn by the receiver
  as an arrow on top of the stream. It stays sharp and has no encoder latency. It is hidden while the pointer is on another display.
- Start the receiver with `-fullscreen`, or press `F11`, `F` or double-click to toggle fullscreen. `Esc` leaves fullscreen.
- The sender retries until the receiver is reachable, and exits once the receiver disconnects it.

## Releases

Pushing a `v*` tag builds `sender` and `receiver` for Windows (amd64) and macOS (arm64, amd64)
and publishes them as a GitHub release:

```sh
git tag v0.1.0 && git push origin v0.1.0
```

On macOS, grant the sender Screen Recording permission (System Settings → Privacy & Security).
Downloaded binaries are unsigned, so remove the quarantine flag first: `xattr -d com.apple.quarantine ./sender-darwin-arm64`.

## Protocol

Every message is a 1-byte type, a 4-byte big-endian payload length, and the payload.

| Type | Direction | Payload |
| --- | --- | --- |
| `Hello` | sender → receiver | JSON `{"codec":"h264","width":1920,"height":1080}`, sent once first |
| `Video` | sender → receiver | Chunk of the raw Annex-B bitstream (not aligned to frames) |
| `Cursor` | sender → receiver | int32 x, int32 y (frame pixels, big-endian), 1-byte visible flag |

## Development

```sh
go test ./...          # includes an ffmpeg encode/decode latency test (skipped if ffmpeg is missing)
go test -short ./...   # skips the ffmpeg integration test
```

## Notes

- Allow inbound TCP on the receiver's port in Windows Firewall (Windows prompts on first run).
- Traffic is unencrypted and unauthenticated. Use it only on trusted networks.
