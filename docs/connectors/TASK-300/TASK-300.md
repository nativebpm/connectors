---
task: TASK-300
status: In Progress
summary: Create Go connector and examples for gastongouron/ironpress PDF converter with k6 load tests.
---

# TASK-300: Ironpress Go Connector

## Description
Write a Go connector (client SDK + self-contained HTTP server wrapper) for `ironpress` PDF converter (https://github.com/gastongouron/ironpress).
Provide examples showing how to use the client and run the server.
Write a load test script using `k6` to benchmark the HTTP server under concurrent HTML-to-PDF conversions.
