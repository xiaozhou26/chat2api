# Build stage
FROM rust:1.85-bookworm AS builder

WORKDIR /app

# Install BoringSSL build dependencies
RUN apt-get update && apt-get install -y build-essential cmake perl pkg-config libclang-dev musl-tools git -y

# Copy manifests
COPY Cargo.toml Cargo.lock ./

# Create a dummy main.rs to cache dependencies
RUN mkdir src && echo "fn main() {}" > src/main.rs

# Build dependencies only (cached layer)
RUN cargo build --release 2>/dev/null || true

# Copy the real source code
COPY src/ src/

# Touch main.rs to force rebuild
RUN touch src/main.rs

# Build the application
RUN cargo build --release

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies for BoringSSL
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary
COPY --from=builder /app/target/release/chat2api /app/chat2api

# Expose port
EXPOSE 3040

# Run the binary
CMD ["/app/chat2api"]
