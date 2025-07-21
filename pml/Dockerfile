# Stage 1: Build stage with uv
FROM python:3.13.2-slim as builder

# Install uv
RUN pip install uv

# Copy project files
WORKDIR /app
COPY pyproject.toml pyproject.toml

# Install dependencies for CPU
RUN uv pip install --system --no-cache .[cpu]

# Stage 2: Final stage
FROM python:3.13.2-slim

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
