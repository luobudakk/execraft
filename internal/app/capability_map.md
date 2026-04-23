# Capability Mapping and Rewrite Boundaries

This project is a clean-room rewrite inspired by behavior-level ideas from `execgo`.
No code, comments, function order, naming style, or implementation details are copied.

## Borrowed Product-Level Capabilities

- Accept directed acyclic task graphs.
- Execute tasks with dependency awareness.
- Expose task submission/query over HTTP.
- Provide retries and timeout controls.
- Offer health and runtime metrics.

## Explicit Non-Isomorphic Design Rules

- Use event-driven runtime instead of a direct state mutation scheduler.
- Use worker-pool with bounded queue and backpressure.
- Persist state by append-only event journal plus snapshots.
- Use different package split and domain vocabulary.
- Use fresh transport handlers and response schemas.

## Innovation Commitments

- Stream task updates over SSE.
- Support filtered task list API.
- Provide CLI subcommands (`serve`, `submit`, `watch`) for operator UX.
