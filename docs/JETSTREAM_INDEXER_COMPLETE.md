# Jetstream indexer implementation

This document previously described the retired hand-written Jetstream v1
WebSocket client. The production command now uses the official Jetstream v2 Go
SDK and durable sequence cursors.

Current architecture and operational contracts are documented in
[internal/indexer/README.md](../internal/indexer/README.md) and
[jetstream-reconnection.md](./jetstream-reconnection.md).
