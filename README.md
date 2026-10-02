# screen-sharer

Stream a screen over the LAN as MJPEG over TCP.

- `cmd/receiver`: listens for senders and shows the stream in a resizable window.
  Only one sender is shown at a time. When a new sender connects, the previous one is disconnected.
- `cmd/sender`: connects to a receiver and streams the display it was told to share.

## Usage

On the viewing machine:

```sh
go run ./cmd/receiver -addr :9000
```

On the machine sharing its screen:

```sh
go run ./cmd/sender -addr <receiver-ip>:9000 -display 1 -fps 15 -quality 70
```

- `-display` is 1-based (`1` is the first monitor).
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
The only message type today is `Frame` (sender → receiver, JPEG image).

## Notes

- Allow inbound TCP on the receiver's port in Windows Firewall (Windows prompts on first run).
- Traffic is unencrypted and unauthenticated. Use it only on trusted networks.
