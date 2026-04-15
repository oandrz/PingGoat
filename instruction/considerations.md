# Technology Considerations & Trade-offs

## Go Buffered Channel vs RabbitMQ (Job Queue)

### Decision: Use Go Buffered Channels

For DocGoat's async job processing, we use Go's built-in buffered channels instead of an external message broker like RabbitMQ.

### Comparison

| | Go Buffered Channel | RabbitMQ |
|---|---|---|
| **Infrastructure** | Zero — built into the language | Requires a separate broker service (another container, config, monitoring) |
| **Complexity** | ~5 lines of code | Client library, connection management, reconnect logic, ack/nack handling |
| **Persistence** | In-memory only (lost on restart) | Persists messages to disk |
| **Distributed** | Single process only | Multiple consumers across machines |
| **Learning value** | Teaches Go concurrency primitives (goroutines, channels, select) | Teaches message broker patterns |

### Why Channels Win for This Project

1. **Single-process app.** The API server and workers run in the same Go binary. No need to send messages across machines. Channels are literally designed for goroutine-to-goroutine communication within one process.

2. **Durability is already handled by Postgres.** Jobs are inserted into the DB with status `"pending"` *before* being sent to the channel. If the process crashes, no jobs are lost — on restart, a recovery query (`SELECT * FROM jobs WHERE status = 'pending'`) can re-enqueue them. The DB is the durability layer, not the queue.

3. **Learning Go concurrency is the goal.** This is a capstone project. The whole point is to use goroutines, channels, `select`, `sync.WaitGroup`, and context cancellation. Swapping in RabbitMQ would skip the most valuable learning.

4. **Throughput is tiny.** Gemini rate limits are 10 RPM / 250 RPD. A buffered channel with a handful of workers is more than enough.

### When Would You Use RabbitMQ Instead?

Reach for a message broker when:
- You have **multiple separate services** that need to communicate (microservices architecture)
- You need **guaranteed delivery** across process boundaries
- You need **fan-out to many consumers** on different machines
- Your throughput requires **horizontal scaling** beyond one process

None of those apply to DocGoat.
