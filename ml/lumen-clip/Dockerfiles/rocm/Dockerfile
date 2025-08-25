# Stage 1: Build stage with uv
FROM rocm/pytorch:rocm6.3_ubuntu22.04_py3.10_pytorch_release-2.5.0 as builder

# Update to Python 3.13
RUN apt-get update && \
    apt-get install -y software-properties-common && \
    add-apt-repository ppa:deadsnakes/ppa && \
    apt-get update && \
    apt-get install -y python3.13 python3.13-pip python3.13-venv python3.13-dev && \
    update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.13 1 && \
    python3 -m pip install --upgrade pip && \
    pip install uv

# Copy project files
WORKDIR /app
COPY pyproject.toml pyproject.toml

# Install dependencies for ROCM
RUN uv pip install --system --no-cache --index-url https://download.pytorch.org/whl/rocm6.3 .[gpu]

# Stage 2: Final stage
FROM rocm/pytorch:rocm6.3_ubuntu22.04_py3.10_pytorch_release-2.5.0

WORKDIR /app

# Update to Python 3.13 in final stage
RUN apt-get update && \
    apt-get install -y software-properties-common && \
    add-apt-repository ppa:deadsnakes/ppa && \
    apt-get update && \
    apt-get install -y python3.13 python3.13-pip python3.13-venv python3.13-dev && \
    update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.13 1

# Copy installed dependencies from builder
COPY --from=builder /usr/local/lib/python3.13/site-packages /usr/local/lib/python3.13/site-packages

# Copy application code
COPY . .

# Set environment variables
ENV PYTHONPATH=/app
ENV HSA_OVERRIDE_GFX_VERSION=10.3.0

# Expose the gRPC port
EXPOSE 50051

# Run the server
CMD ["python3", "server.py"]
