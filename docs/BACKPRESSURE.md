# Jetstream v2 flow control

The retired v1 client maintained its own buffered queue and pause thresholds.
Jetstream v2 delivers bounded SDK batches directly to the synchronous database
projector. The consumer does not request the next batch until the current batch
and cursor commit together, so the database transaction is the flow-control
boundary and there is no second in-process record queue.

Use `JETSTREAM_BATCH_SIZE` to tune the maximum SDK batch size. Archive download
and retry concurrency remain SDK-owned behavior.
