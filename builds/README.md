# Builds

Ready-to-run builds of Googleplex **1.0.0**. They aren't code-signed; see the [main README](../README.md#download) for how to open them the first time.

| File | System | Size | SHA-256 |
|---|---|---|---|
| `Googleplex-mac-arm64.dmg` | macOS 11+ on Apple Silicon (M1 and later) | 7.3 MB | `0277db3ab3d9c71987cfd1c56801effeb0fd4ff0c99e325f33d12bdbff7a4954` |
| `Googleplex-windows-amd64.exe` | Windows 10/11, 64-bit | 17.8 MB | `639f13e3df11958dfc400eeb3abc4d0c445d1e75ecceb15fbc89b2290435af96` |

- **macOS:** open the .dmg and drag Googleplex into Applications.
- **Windows:** this is the app itself, not an installer, so just run it. It needs Microsoft's WebView2 runtime, which Windows 11 and up-to-date Windows 10 already have; if it's missing, the app offers to download it.
- **Linux:** there's no prebuilt file. Build from source with `./scripts/build-linux.sh`, which makes a binary and a .deb.

Check a download:

```bash
shasum -a 256 Googleplex-mac-arm64.dmg                 # macOS
certutil -hashfile Googleplex-windows-amd64.exe SHA256   # Windows
```
