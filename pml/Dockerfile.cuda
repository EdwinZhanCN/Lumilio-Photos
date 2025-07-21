# Stage 1: Build stage with uv
FROM nvidia/cuda:12.6-base-ubuntu22.04 as builder

# Install Python and uv
RUN apt-get update && \
    apt-get install -y python3.13 python3-pip && \
    pip install uv

# Copy project files
WORKDIR /app
COPY pyproject.toml pyproject.toml

# Install dependencies for GPU
RUN uv pip install --system --no-cache --index-url https://download.pytorch.org/whl/cu126 .[gpu]

# Stage 2: Final stage
FROM nvidia/cuda:12.6-base-ubuntu22.04

WORKDIR /app

# Copy installed dependencies from builder
COPY --from=builder /usr/local/lib/python3.13/site-packages /usr/local/lib/python3.13/site-packages

# Copy application code
COPY . .

# Set environment variables
ENV PYTHONPATH=/app

# Expose the gRPC port
EXPOSE 50051

# Run the server
CMD ["python", "server.py"]
